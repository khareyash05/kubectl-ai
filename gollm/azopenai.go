// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gollm

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"k8s.io/klog/v2"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cognitiveservices/armcognitiveservices"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
)

func init() {
	if err := RegisterProvider("azopenai", azureOpenAIFactory); err != nil {
		klog.Fatalf("Failed to register azopenai provider: %v", err)
	}
}

/*
azureOpenAIFactory is the provider factory function for Azure OpenAI.
Supports ClientOptions for custom configuration.
*/
func azureOpenAIFactory(ctx context.Context, opts ClientOptions) (Client, error) {
	return NewAzureOpenAIClient(ctx, opts)
}

type AzureOpenAIClient struct {
	client   *azopenai.Client
	endpoint string
}

var _ Client = &AzureOpenAIClient{}

// NewAzureOpenAIClient creates a new Azure OpenAI client.
// Supports ClientOptions and SkipVerifySSL for custom HTTP transport.
func NewAzureOpenAIClient(ctx context.Context, opts ClientOptions) (*AzureOpenAIClient, error) {
	azureOpenAIEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	if opts.URL != nil && opts.URL.Host != "" {
		opts.URL.Scheme = "https"
		azureOpenAIEndpoint = opts.URL.String()
	}
	if azureOpenAIEndpoint == "" {
		return nil, fmt.Errorf("AZURE_OPENAI_ENDPOINT environment variable not set")
	}
	azureOpenAIClient := AzureOpenAIClient{
		endpoint: azureOpenAIEndpoint,
	}

	// Create a custom HTTP client (supports SkipVerifySSL)
	httpClient := createCustomHTTPClient(opts.SkipVerifySSL)

	azureOpenAIKey := os.Getenv("AZURE_OPENAI_API_KEY")
	clientOpts := &azopenai.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: httpClient,
		},
	}
	if azureOpenAIKey != "" {
		keyCredential := azcore.NewKeyCredential(azureOpenAIKey)
		client, err := azopenai.NewClientWithKeyCredential(azureOpenAIEndpoint, keyCredential, clientOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to create azure openai client: %w", err)
		}
		azureOpenAIClient.client = client
	} else {
		credential, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get credential: %w", err)
		}
		client, err := azopenai.NewClient(azureOpenAIEndpoint, credential, clientOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to create azure openai client: %w", err)
		}
		azureOpenAIClient.client = client
	}

	return &azureOpenAIClient, nil
}

func (c *AzureOpenAIClient) Close() error {
	return nil
}

func (c *AzureOpenAIClient) GenerateCompletion(ctx context.Context, request *CompletionRequest) (CompletionResponse, error) {
	req := azopenai.ChatCompletionsOptions{
		Messages: []azopenai.ChatRequestMessageClassification{
			&azopenai.ChatRequestUserMessage{Content: azopenai.NewChatRequestUserMessageContent(request.Prompt)},
		},
		DeploymentName: &request.Model,
	}

	resp, err := c.client.GetChatCompletions(ctx, req, nil)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil || resp.Choices[0].Message.Content == nil {
		return nil, fmt.Errorf("invalid completion response: %v", resp)
	}

	return &AzureOpenAICompletionResponse{response: *resp.Choices[0].Message.Content}, nil
}

func (c *AzureOpenAIClient) ListModels(ctx context.Context) ([]string, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	subClient, err := armsubscription.NewSubscriptionsClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	subPager := subClient.NewListPager(nil)
	for subPager.More() {
		subResp, err := subPager.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get subscriptions page: %w", err)
		}

		for _, sub := range subResp.Value {
			accountClient, err := armcognitiveservices.NewAccountsClient(*sub.SubscriptionID, cred, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create accounts client: %w", err)
			}

			accountPager := accountClient.NewListPager(nil)
			for accountPager.More() {
				accountResp, err := accountPager.NextPage(context.Background())
				if err != nil {
					return nil, fmt.Errorf("failed to to get accounts page: %w", err)
				}

				for _, account := range accountResp.Value {
					if account.Kind == nil || !slices.Contains([]string{"OpenAI", "CognitiveServices", "AIServices"}, *account.Kind) {
						// Not an Azure OpenAI service
						continue
					}
					if account.Properties == nil || account.Properties.Endpoint == nil || strings.TrimSuffix(*account.Properties.Endpoint, "/") != c.endpoint {
						// Not the expected endpoint
						continue
					}

					resourceID, err := arm.ParseResourceID(*account.ID)
					if err != nil {
						return nil, fmt.Errorf("failed to parse resource ID %q: %w", *account.Name, err)
					}

					deploymentClient, err := armcognitiveservices.NewDeploymentsClient(*sub.SubscriptionID, cred, nil)
					if err != nil {
						return nil, fmt.Errorf("failed to create deployments client: %w", err)
					}

					var modelNames []string
					deploymentPager := deploymentClient.NewListPager(resourceID.ResourceGroupName, *account.Name, nil)
					for deploymentPager.More() {
						deploymentResp, err := deploymentPager.NextPage(context.Background())
						if err != nil {
							return nil, fmt.Errorf("failed to get deployments page: %w", err)
						}

						for _, deployment := range deploymentResp.Value {
							modelNames = append(modelNames, *deployment.Name)
						}

					}
					slices.Sort(modelNames)
					return modelNames, nil
				}
			}
		}
	}

	return nil, nil
}

func (c *AzureOpenAIClient) SetResponseSchema(schema *Schema) error {
	return nil
}

func (c *AzureOpenAIClient) StartChat(systemPrompt string, model string) Chat {
	var a Chat
	return a
}

type AzureOpenAICompletionResponse struct {
	response string
}

func (r *AzureOpenAICompletionResponse) Response() string {
	return r.response
}

func (r *AzureOpenAICompletionResponse) UsageMetadata() any {
	return nil
}
