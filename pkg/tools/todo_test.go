package tools

import (
	"testing"
)

func TestTodoItem_Validation(t *testing.T) {
	tests := []struct {
		name    string
		item    TodoItem
		wantErr bool
	}{
		{
			name: "valid todo item",
			item: TodoItem{
				ID:       "1",
				Content:  "Test task",
				Status:   StatusPending,
				Priority: PriorityHigh,
			},
			wantErr: false,
		},
		{
			name: "empty content should fail",
			item: TodoItem{
				ID:       "1",
				Content:  "",
				Status:   StatusPending,
				Priority: PriorityHigh,
			},
			wantErr: true,
		},
		{
			name: "empty ID should fail",
			item: TodoItem{
				ID:       "",
				Content:  "Test task",
				Status:   StatusPending,
				Priority: PriorityHigh,
			},
			wantErr: true,
		},
		{
			name: "invalid status should fail",
			item: TodoItem{
				ID:       "1",
				Content:  "Test task",
				Status:   "invalid",
				Priority: PriorityHigh,
			},
			wantErr: true,
		},
		{
			name: "invalid priority should fail",
			item: TodoItem{
				ID:       "1",
				Content:  "Test task",
				Status:   StatusPending,
				Priority: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("TodoItem.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"pending is valid", StatusPending, true},
		{"in_progress is valid", StatusInProgress, true},
		{"completed is valid", StatusCompleted, true},
		{"invalid status", "invalid", false},
		{"empty status", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriority_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		want     bool
	}{
		{"high is valid", PriorityHigh, true},
		{"medium is valid", PriorityMedium, true},
		{"low is valid", PriorityLow, true},
		{"invalid priority", "invalid", false},
		{"empty priority", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.priority.IsValid(); got != tt.want {
				t.Errorf("Priority.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}