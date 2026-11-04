package bot

import (
	"reflect"
	"testing"
)

func TestParseMessageContent_PlainText(t *testing.T) {
	result := ParseMessageContent("こんにちは、システムを再起動してください。")
	if result.IsCommand {
		t.Fatal("expected IsCommand to be false")
	}
	if result.Text != "こんにちは、システムを再起動してください。" {
		t.Fatalf("expected text to match, got %q", result.Text)
	}
}

func TestParseMessageContent_CommandWithArg(t *testing.T) {
	result := ParseMessageContent("/mode gpt-4")
	if !result.IsCommand {
		t.Fatal("expected IsCommand to be true")
	}
	if result.CommandName != "mode" {
		t.Fatalf("expected commandName 'mode', got %q", result.CommandName)
	}
	expectedArgs := []string{"gpt-4"}
	if !reflect.DeepEqual(result.Args, expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, result.Args)
	}
}

func TestParseMessageContent_DoubleQuotedArgs(t *testing.T) {
	result := ParseMessageContent(`/template add "朝会" "おはよう"`)
	if !result.IsCommand {
		t.Fatal("expected IsCommand to be true")
	}
	if result.CommandName != "template" {
		t.Fatalf("expected commandName 'template', got %q", result.CommandName)
	}
	expectedArgs := []string{"add", "朝会", "おはよう"}
	if !reflect.DeepEqual(result.Args, expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, result.Args)
	}
}

func TestParseMessageContent_UnquotedArgs(t *testing.T) {
	result := ParseMessageContent("/template PR作成")
	if !result.IsCommand {
		t.Fatal("expected IsCommand to be true")
	}
	if result.CommandName != "template" {
		t.Fatalf("expected commandName 'template', got %q", result.CommandName)
	}
	expectedArgs := []string{"PR作成"}
	if !reflect.DeepEqual(result.Args, expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, result.Args)
	}
}

func TestParseMessageContent_SurroundingWhitespace(t *testing.T) {
	result := ParseMessageContent("  /stop  ")
	if !result.IsCommand {
		t.Fatal("expected IsCommand to be true")
	}
	if result.CommandName != "stop" {
		t.Fatalf("expected commandName 'stop', got %q", result.CommandName)
	}
	if len(result.Args) != 0 {
		t.Fatalf("expected empty args, got %v", result.Args)
	}
}

func TestParseMessageContent_LoneSlash(t *testing.T) {
	result := ParseMessageContent("/")
	if result.IsCommand {
		t.Fatal("expected IsCommand to be false")
	}
	if result.Text != "/" {
		t.Fatalf("expected text '/', got %q", result.Text)
	}
}
