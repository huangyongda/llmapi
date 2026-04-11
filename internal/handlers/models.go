package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewModelsHandler() *ModelsHandler {
	return &ModelsHandler{}
}

type ModelsHandler struct{}

// 模型信息结构体
type Model struct {
	ID                     string   `json:"id"`
	Object                 string   `json:"object"`
	Created                int64    `json:"created"`
	OwnedBy                string   `json:"owned_by"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types"`
}

// 响应结构体
type ModelsListResponse struct {
	Data    []Model `json:"data"`
	Object  string  `json:"object"`
	Success bool    `json:"success"`
}

func (h *ModelsHandler) GetModels(c *gin.Context) {
	// 构建响应
	var modelList []Model
	modelMapping := map[string]string{
		"MiniMax-M2":   "custom",
		"MiniMax-M2.1": "custom",
		"MiniMax-M2.5": "custom",
		"MiniMax-M2.7": "custom",
	}
	level, _ := c.Get("level")
	if level == 2 {
		highspeed := map[string]string{
			"MiniMax-M2.7-highspeed": "custom",
			"MiniMax-M2.5-highspeed": "custom",
			"MiniMax-M2.1-highspeed": "custom",
		}
		for key, val := range highspeed {
			modelMapping[key] = val
		}

	}

	useGlm, _ := c.Get("UseGlm")
	if useGlm == 1 {
		glmModels := map[string]string{
			"GLM-4.6":                    "custom",
			"GLM-4.6V-FlashX":            "custom",
			"GLM-4.7":                    "custom",
			"GLM-Image":                  "custom",
			"GLM-5-Turbo":                "custom",
			"GLM-5V-Turbo":               "custom",
			"GLM-5.1":                    "custom",
			"GLM-4.5":                    "custom",
			"GLM-4.6V":                   "custom",
			"GLM-4.7-Flash":              "custom",
			"GLM-4.7-FlashX":             "custom",
			"GLM-OCR":                    "custom",
			"GLM-5":                      "custom",
			"GLM-4-Plus":                 "custom",
			"GLM-4.5V":                   "custom",
			"GLM-4.6V-Flash":             "custom",
			"AutoGLM-Phone-Multilingual": "custom",
			"GLM-4.5-Air":                "custom",
			"GLM-4.5-AirX":               "custom",
			"GLM-4.5-Flash":              "custom",
			"GLM-4-32B-0414-128K":        "custom",
			"CogView-4-250304":           "custom",
			"GLM-ASR-2512":               "custom",
			"ViduQ1-text":                "custom",
			"Viduq1-Image":               "custom",
			"Viduq1-Start-End":           "custom",
			"Vidu2-Image":                "custom",
			"Vidu2-Start-End":            "custom",
			"Vidu2-Reference":            "custom",
			"CogVideoX-3":                "custom",
		}
		for key, val := range glmModels {
			modelMapping[key] = val
		}

	}

	createdTimestamp := int64(1626777600)

	for modelID, owner := range modelMapping {
		model := Model{
			ID:                     modelID,
			Object:                 "model",
			Created:                createdTimestamp,
			OwnedBy:                owner,
			SupportedEndpointTypes: []string{"openai"},
		}
		modelList = append(modelList, model)
	}

	response := ModelsListResponse{
		Data:    modelList,
		Object:  "list",
		Success: true,
	}

	// 返回JSON响应
	c.JSON(http.StatusOK, response)
}
