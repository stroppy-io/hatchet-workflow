package domain

type FailureInput struct {
	Message     string `json:"message"`
	ShouldFail  bool   `json:"should_fail"`
	FailureType string `json:"failure_type"`
}

type TaskOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type FailureHandlerOutput struct {
	FailureHandled bool   `json:"failure_handled"`
	ErrorDetails   string `json:"error_details"`
	OriginalInput  string `json:"original_input"`
}
