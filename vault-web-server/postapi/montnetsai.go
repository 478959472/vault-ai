package postapi

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pashpashpash/vault/chunk"
	"github.com/pkoukk/tiktoken-go"
	openai "github.com/sashabaranov/go-openai"
)

type MontnetsAIResponse struct {
	Response string `json:"response"`
	Tokens   int    `json:"tokens"`
}

func callMontnetsAI(client *openai.Client, prompt string, model string,
	instructions string, maxTokens int) (string, int, error) {
	// set request details
	temperature := float32(0.7)
	topP := float32(1.0)
	frequencyPenalty := float32(0.0)
	presencePenalty := float32(0.6)
	stop := []string{"Human:", "AI:"}

	var assistantMessage string
	var tokens int
	var err error
	assistantMessage, tokens, err = useMontnetsChatCompletionAPI(client, prompt, model, instructions, temperature,
		maxTokens, topP, frequencyPenalty, presencePenalty, stop)

	return assistantMessage, tokens, err
}

func useMontnetsChatCompletionAPI(client *openai.Client, prompt, modelParam string, instructions string, temperature float32, maxTokens int, topP float32, frequencyPenalty, presencePenalty float32, stop []string) (string, int, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: instructions,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:            modelParam,
			Messages:         messages,
			Temperature:      temperature,
			MaxTokens:        maxTokens,
			TopP:             topP,
			FrequencyPenalty: frequencyPenalty,
			PresencePenalty:  presencePenalty,
			Stop:             stop,
		},
	)

	if err != nil {
		return "", 0, err
	}

	return resp.Choices[0].Message.Content, resp.Usage.TotalTokens, nil
}

func callMontnetsEmbeddingAPIWithRetry(client *openai.Client, texts []string, embedModel openai.EmbeddingModel,
	maxRetries int) (*openai.EmbeddingResponse, error) {
	var err error
	var res openai.EmbeddingResponse

	for i := 0; i < maxRetries; i++ {
		res, err = client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
			Input: texts,
			Model: embedModel,
		})

		if err == nil {
			return &res, nil
		}

		time.Sleep(5 * time.Second)
	}

	return nil, err
}

func getMontnetsEmbeddings(client *openai.Client, chunks []chunk.Chunk, batchSize int,
	embedModel openai.EmbeddingModel) ([][]float32, error) {
	embeddings := make([][]float32, 0, len(chunks))

	for i := 0; i < len(chunks); i += batchSize {
		iEnd := min(len(chunks), i+batchSize)

		texts := make([]string, 0, iEnd-i)
		for _, chunk := range chunks[i:iEnd] {
			texts = append(texts, chunk.Text)
		}

		log.Println("[getEmbeddings] Feeding texts to Openai to get embedding...\n", texts)

		res, err := callEmbeddingAPIWithRetry(client, texts, embedModel, 3)
		if err != nil {
			return nil, err
		}

		embeds := make([][]float32, len(res.Data))
		for i, record := range res.Data {
			embeds[i] = record.Embedding
		}

		embeddings = append(embeddings, embeds...)
	}

	return embeddings, nil
}

func getMontnetsEmbedding(client *openai.Client, text string, embedModel openai.EmbeddingModel) ([]float32, error) {
	res, err := callEmbeddingAPIWithRetry(client, []string{text}, embedModel, 3)
	if err != nil {
		return nil, err
	}

	return res.Data[0].Embedding, nil
}

func buildMontnetsPrompt(contexts []string, question string) (string, error) {
	tokenLimit := 3750
	promptStart := "Answer the question based on the context below.\n\nContext:\n"
	promptEnd := fmt.Sprintf("\n\n问题: %s\n使用中文回答:", question)

	// Get tiktoken encoding for the model
	tke, err := tiktoken.EncodingForModel("davinci")
	if err != nil {
		return "", fmt.Errorf("getEncoding: %v", err)
	}

	// Count tokens for the question
	questionTokens := tke.Encode(question, nil, nil)
	currentTokenCount := len(questionTokens)

	var prompt string
	for i := range contexts {
		// Count tokens for the current context
		contextTokens := tke.Encode(contexts[i], nil, nil)
		currentTokenCount += len(contextTokens)

		if currentTokenCount >= tokenLimit {
			prompt = promptStart + strings.Join(contexts[:i], "\n\n---\n\n") + promptEnd
			break
		} else if i == len(contexts)-1 {
			prompt = promptStart + strings.Join(contexts, "\n\n---\n\n") + promptEnd
		}
	}

	return prompt, nil
}
