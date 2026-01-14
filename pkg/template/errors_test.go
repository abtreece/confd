package template

import (
	"errors"
	"testing"
)

func TestParseFailureMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    FailureMode
		wantErr bool
	}{
		{
			name:    "best-effort",
			input:   "best-effort",
			want:    FailModeBestEffort,
			wantErr: false,
		},
		{
			name:    "fail-fast",
			input:   "fail-fast",
			want:    FailModeFast,
			wantErr: false,
		},
		{
			name:    "case insensitive best-effort",
			input:   "BEST-EFFORT",
			want:    FailModeBestEffort,
			wantErr: false,
		},
		{
			name:    "case insensitive fail-fast",
			input:   "FAIL-FAST",
			want:    FailModeFast,
			wantErr: false,
		},
		{
			name:    "invalid mode",
			input:   "invalid",
			want:    FailModeBestEffort,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    FailModeBestEffort,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFailureMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFailureMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseFailureMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFailureMode_String(t *testing.T) {
	tests := []struct {
		name string
		mode FailureMode
		want string
	}{
		{
			name: "best-effort",
			mode: FailModeBestEffort,
			want: "best-effort",
		},
		{
			name: "fail-fast",
			mode: FailModeFast,
			want: "fail-fast",
		},
		{
			name: "invalid",
			mode: FailureMode(99),
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("FailureMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBatchProcessResult_Error(t *testing.T) {
	tests := []struct {
		name    string
		result  *BatchProcessResult
		wantErr bool
		wantNil bool
	}{
		{
			name: "no errors",
			result: &BatchProcessResult{
				Total:     2,
				Succeeded: 2,
				Failed:    0,
				Statuses: []TemplateStatus{
					{Dest: "/tmp/test1", Success: true},
					{Dest: "/tmp/test2", Success: true},
				},
			},
			wantErr: false,
			wantNil: true,
		},
		{
			name: "single error",
			result: &BatchProcessResult{
				Total:     2,
				Succeeded: 1,
				Failed:    1,
				Statuses: []TemplateStatus{
					{Dest: "/tmp/test1", Success: true},
					{Dest: "/tmp/test2", Error: errors.New("template error")},
				},
			},
			wantErr: true,
			wantNil: false,
		},
		{
			name: "multiple errors",
			result: &BatchProcessResult{
				Total:     3,
				Succeeded: 1,
				Failed:    2,
				Statuses: []TemplateStatus{
					{Dest: "/tmp/test1", Success: true},
					{Dest: "/tmp/test2", Error: errors.New("error 1")},
					{Dest: "/tmp/test3", Error: errors.New("error 2")},
				},
			},
			wantErr: true,
			wantNil: false,
		},
		{
			name: "all errors",
			result: &BatchProcessResult{
				Total:     2,
				Succeeded: 0,
				Failed:    2,
				Statuses: []TemplateStatus{
					{Dest: "/tmp/test1", Error: errors.New("error 1")},
					{Dest: "/tmp/test2", Error: errors.New("error 2")},
				},
			},
			wantErr: true,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Error()
			if tt.wantNil {
				if err != nil {
					t.Errorf("BatchProcessResult.Error() = %v, want nil", err)
				}
			} else {
				if (err != nil) != tt.wantErr {
					t.Errorf("BatchProcessResult.Error() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestBatchProcessResult_Error_JoinedErrors(t *testing.T) {
	// Test that errors.Join is used and individual errors can be extracted
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	result := &BatchProcessResult{
		Total:     2,
		Succeeded: 0,
		Failed:    2,
		Statuses: []TemplateStatus{
			{Dest: "/tmp/test1", Error: err1},
			{Dest: "/tmp/test2", Error: err2},
		},
	}

	joinedErr := result.Error()
	if joinedErr == nil {
		t.Fatal("BatchProcessResult.Error() returned nil, expected error")
	}

	// Verify that the joined error contains both original errors
	if !errors.Is(joinedErr, err1) {
		t.Errorf("Joined error should contain err1")
	}
	if !errors.Is(joinedErr, err2) {
		t.Errorf("Joined error should contain err2")
	}
}
