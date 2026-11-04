package bot

import (
	"fmt"
	"strings"

	"antigravity-telegram-remote/internal/database"
	"antigravity-telegram-remote/internal/i18n"
)

// CommandResult represents the result of executing a command handled by SlashCommandHandler.
type CommandResult struct {
	Success bool
	Message string
	Prompt  string
}

type templateStore interface {
	FindAll() ([]database.TemplateRecord, error)
	FindByName(name string) (*database.TemplateRecord, error)
	Create(input database.CreateTemplateInput) (database.TemplateRecord, error)
	DeleteByName(name string) (bool, error)
}

// SlashCommandHandler handles slash commands such as /template.
type SlashCommandHandler struct {
	templateRepo templateStore
}

// NewSlashCommandHandler creates a new SlashCommandHandler with the provided repository.
func NewSlashCommandHandler(templateRepo *database.TemplateRepository) *SlashCommandHandler {
	return &SlashCommandHandler{
		templateRepo: templateRepo,
	}
}

// NewSlashCommandHandlerWithStore creates a SlashCommandHandler with any templateStore (useful for mocks).
func NewSlashCommandHandlerWithStore(store templateStore) *SlashCommandHandler {
	return &SlashCommandHandler{
		templateRepo: store,
	}
}

// HandleCommand processes slash command routing and execution.
func (h *SlashCommandHandler) HandleCommand(commandName string, args []string) CommandResult {
	switch strings.ToLower(commandName) {
	case "template":
		return h.handleTemplateCommand(args)
	default:
		return CommandResult{
			Success: false,
			Message: i18n.T(fmt.Sprintf("⚠️ Unknown command: /%s", commandName)),
		}
	}
}

func (h *SlashCommandHandler) handleTemplateCommand(args []string) CommandResult {
	if len(args) == 0 {
		templates, err := h.templateRepo.FindAll()
		if err != nil || len(templates) == 0 {
			return CommandResult{
				Success: true,
				Message: i18n.T("📝 No templates registered."),
			}
		}

		lines := make([]string, 0, len(templates))
		for _, t := range templates {
			lines = append(lines, fmt.Sprintf("- **%s**", t.Name))
		}
		list := strings.Join(lines, "\n")
		return CommandResult{
			Success: true,
			Message: i18n.T(fmt.Sprintf("📝 Registered Templates:\n%s\n\nTo use: `/template [name]`", list)),
		}
	}

	subCommandOrName := args[0]

	// add: register new template
	if strings.ToLower(subCommandOrName) == "add" {
		if len(args) < 3 {
			return CommandResult{
				Success: false,
				Message: i18n.T("⚠️ Missing arguments.\nUsage: `/template add \"name\" \"prompt\"`"),
			}
		}
		name := args[1]
		prompt := strings.Join(args[2:], " ")

		_, err := h.templateRepo.Create(database.CreateTemplateInput{
			Name:   name,
			Prompt: prompt,
		})
		if err != nil {
			return CommandResult{
				Success: false,
				Message: i18n.T("⚠️ Failed to register template. Name might be duplicated."),
			}
		}
		return CommandResult{
			Success: true,
			Message: i18n.T(fmt.Sprintf("✅ Template **%s** registered.", name)),
		}
	}

	// delete: remove template
	if strings.ToLower(subCommandOrName) == "delete" {
		if len(args) < 2 {
			return CommandResult{
				Success: false,
				Message: i18n.T("⚠️ Specify a template name to delete.\nUsage: `/template delete \"name\"`"),
			}
		}
		name := args[1]
		deleted, err := h.templateRepo.DeleteByName(name)
		if err == nil && deleted {
			return CommandResult{
				Success: true,
				Message: i18n.T(fmt.Sprintf("🗑️ Template **%s** deleted.", name)),
			}
		}
		return CommandResult{
			Success: false,
			Message: i18n.T(fmt.Sprintf("⚠️ Template **%s** not found.", name)),
		}
	}

	// Otherwise treat as template invocation
	templateName := subCommandOrName
	tmpl, err := h.templateRepo.FindByName(templateName)
	if err != nil || tmpl == nil {
		return CommandResult{
			Success: false,
			Message: i18n.T(fmt.Sprintf("⚠️ Template **%s** not found.", templateName)),
		}
	}

	return CommandResult{
		Success: true,
		Message: i18n.T(fmt.Sprintf("📝 Invoked template **%s**.\nStarting process with this prompt.", templateName)),
		Prompt:  tmpl.Prompt,
	}
}
