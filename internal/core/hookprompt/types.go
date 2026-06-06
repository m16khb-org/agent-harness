package hookprompt

type Hint struct {
	Tool     string `json:"tool"`
	Reason   string `json:"reason"`
	Priority string `json:"priority,omitempty"`
}

const (
	PriorityRequired  = "required"
	PriorityConsider  = "consider"
	PriorityRoute     = "route"
	PriorityAction    = "action"
	PrioritySecondary = "secondary"
)
