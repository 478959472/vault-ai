package postapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/pashpashpash/vault/form"
	"github.com/pashpashpash/vault/vectordb"
	openai "github.com/sashabaranov/go-openai"
)

type Context struct {
	Text  string `json:"text"`
	Title string `json:"title"`
}

type Answer struct {
	Answer  string    `json:"answer"`
	Context []Context `json:"context"`
	Tokens  int       `json:"tokens"`
}

// Handle Requests For Question
func (ctx *HandlerContext) QuestionHandler(w http.ResponseWriter, r *http.Request) {
	form := new(form.QuestionForm)

	if errs := FormParseVerify(form, "QuestionForm", w, r); errs != nil {
		return
	}
	form.UUID = "8078d199-aac0-452d-8487-698fc10d3c86"
	log.Println("[QuestionHandler] Question:", form.Question)
	log.Println("[QuestionHandler] Model:", form.Model)
	log.Println("[QuestionHandler] UUID:", form.UUID)
	log.Println("[QuestionHandler] ApiKey:", form.ApiKey)

	clientToUse := ctx.openAIClient
	if form.ApiKey != "" {
		log.Println("[QuestionHandler] Using provided custom API key:", form.ApiKey)
		// openaiConfig := openai.DefaultConfig(form.ApiKey)
		// openaiConfig.BaseURL = "http://94.74.89.252:7758/5g-openai/v1"
		// clientToUse := openai.NewClientWithConfig(openaiConfig)
		// clientToUse = openai.NewClient(form.ApiKey)
	}
	promptMake := ""
	contextAll := make([]Context, 0)
	matchList := []float32{}
	// 判断字符串是否以"//"开头
	if strings.HasPrefix(form.Question, "//") {
		// 去掉开头的"//"
		form.Question = form.Question[2:]
		promptMake = form.Question
	} else {
		// step 1: Feed question to openai embeddings api to get an embedding back
		questionEmbedding, err := getEmbedding(clientToUse, form.Question, openai.AdaEmbeddingV2)
		if err != nil {
			log.Println("[QuestionHandler ERR] OpenAI get embedding request error\n", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("[QuestionHandler] Question Embedding Length:", len(questionEmbedding))

		// step 2: Query vector db using questionEmbedding to get context matches
		matches, err := ctx.vectorDB.Retrieve(questionEmbedding, 4, form.UUID)
		if err != nil {
			log.Println("[QuestionHandler ERR] Vector DB query error\n", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Println("[QuestionHandler] Got matches from vector DB:", matches)
		var filteredMatches []vectordb.QueryMatch
		for _, match := range matches {
			matchList = append(matchList, match.Score)
			if match.Score > 0.8 {
				filteredMatches = append(filteredMatches, match)
			}
		}
		// Extract context text and titles from the matches
		contexts := make([]Context, len(filteredMatches))
		for i, match := range filteredMatches {
			contexts[i].Text = match.Metadata["text"]
			contexts[i].Title = match.Metadata["title"]
		}
		log.Println("[QuestionHandler] Retrieved context from vector DB:\n", contexts)

		// step 3: Structure the prompt with a context section + question, using top x results from vector DB as the context
		contextTexts := make([]string, len(contexts))
		for i, context := range contexts {
			contextTexts[i] = context.Text
		}
		prompt, err := buildPrompt(contextTexts, form.Question)
		log.Println("[QuestionHandler] buildPrompt:\n", prompt)
		if prompt == "" {
			prompt = form.Question + " 使用中文回答"
		}
		promptMake = prompt
		contextAll = contexts
		if err != nil {
			log.Println("[QuestionHandler ERR] Error building prompt\n", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	model := openai.GPT3Dot5Turbo
	if form.Model == "GPT Davinci" {
		model = openai.GPT3TextDavinci003
	}

	log.Printf("[QuestionHandler] Sending OpenAI api request...\nPrompt:%s\n", promptMake)
	openAIResponse, tokens, err := callOpenAI(clientToUse, promptMake, model,
		"You are a helpful assistant answering questions based on the context provided.",
		512)

	if err != nil {
		log.Println("[QuestionHandler ERR] OpenAI answer questions request error\n", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("[QuestionHandler] OpenAI response:\n", openAIResponse)
	response := OpenAIResponse{openAIResponse, tokens}

	log.Println("[QuestionHandler] Query vector from vector DB matchList\n", matchList)

	answer := Answer{response.Response, contextAll, response.Tokens}
	jsonResponse, err := json.Marshal(answer)
	if err != nil {
		log.Println("[QuestionHandler ERR] OpenAI response marshalling error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
}
