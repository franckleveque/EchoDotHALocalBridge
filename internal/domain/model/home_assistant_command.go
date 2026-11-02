package model

type HomeAssistantCommand struct {
	Service string
	Data    HAFields
	Effect  string
}
