package precompile

import "testing"

func TestParseCargoCompilerMessage(t *testing.T) {
	input := `{"reason":"compiler-message","message":{"message":"cannot find value ` + "`x`" + ` in this scope","code":{"code":"E0425"},"level":"error","spans":[{"file_name":"src/main.rs","line_start":3,"column_start":5,"is_primary":true}],"rendered":"error[E0425]"}}`
	sum := ParseDiagnostics(input)
	if sum.Errors != 1 || len(sum.Messages) != 1 {
		t.Fatalf("expected one error, got %+v", sum)
	}
	d := sum.Messages[0]
	if d.Code != "E0425" || d.File != "src/main.rs" || d.Line != 3 || d.Column != 5 {
		t.Fatalf("unexpected diagnostic: %+v", d)
	}
}

func TestParseIgnoresNonJSONLines(t *testing.T) {
	sum := ParseDiagnostics("Compiling demo\n{\"level\":\"warning\",\"message\":\"careful\"}\n")
	if sum.NonJSONLines != 1 || sum.Warnings != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
}
