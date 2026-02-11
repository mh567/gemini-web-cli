package api

import "github.com/harris/gemini-web-cli/internal/config"

// Model represents a Gemini model configuration.
type Model struct {
	Name        string
	DisplayName string
	HeaderVal   string // x-goog-ext-525001261-jspb value
}

// Available models
// Header format: [1,null,null,null,"<model_hash>",null,null,0,[4],null,null,1]
// Hashes from HanaokaYuzu/Gemini-API Python library (gemini-3.0 series)
var Models = map[string]Model{
	"default": {
		Name:        "default",
		DisplayName: "Default (server picks)",
		HeaderVal:   "",
	},
	"gemini-3.0-pro": {
		Name:        "gemini-3.0-pro",
		DisplayName: "Gemini 3.0 Pro",
		HeaderVal:   `[1,null,null,null,"9d8ca3786ebdfbea",null,null,0,[4],null,null,1]`,
	},
	"gemini-3.0-flash": {
		Name:        "gemini-3.0-flash",
		DisplayName: "Gemini 3.0 Flash",
		HeaderVal:   `[1,null,null,null,"fbb127bbb056c959",null,null,0,[4],null,null,1]`,
	},
	"gemini-3.0-flash-thinking": {
		Name:        "gemini-3.0-flash-thinking",
		DisplayName: "Gemini 3.0 Flash Thinking",
		HeaderVal:   `[1,null,null,null,"5bf011840784117a",null,null,0,[4],null,null,1]`,
	},
}

func DefaultModel() Model {
	return Models["default"]
}

// GetModel looks up a model by name from built-in models first,
// then falls back to custom models from config.
func GetModel(name string) (Model, bool) {
	if m, ok := Models[name]; ok {
		return m, true
	}
	return getCustomModel(name)
}

func getCustomModel(name string) (Model, bool) {
	cfg, err := config.Load()
	if err != nil || len(cfg.CustomModels) == 0 {
		return Model{}, false
	}
	cm, ok := cfg.CustomModels[name]
	if !ok {
		return Model{}, false
	}
	return Model{
		Name:        name,
		DisplayName: cm.Name,
		HeaderVal:   cm.HeaderVal,
	}, true
}
