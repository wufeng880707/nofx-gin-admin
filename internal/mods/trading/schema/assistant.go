package schema

import "strings"

type AssistantTaskForm struct {
	Message string `json:"message" binding:"required,max=200"`
}

func (a *AssistantTaskForm) Validate() {
	a.Message = strings.TrimSpace(a.Message)
}

type AssistantTaskView struct {
	Skill string `json:"skill"`
	Text  string `json:"text"`
}
