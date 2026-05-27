package api

// TaskSkill represents a skill included in the task claim response.
type TaskSkill struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Content     string          `json:"content"`
	Files       []TaskSkillFile `json:"files,omitempty"`
}

// TaskSkillFile represents a supporting file within a skill.
type TaskSkillFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
