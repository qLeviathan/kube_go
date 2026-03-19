// Package cli provides an interactive CLI with questionnaire-driven configuration
// for the CLARA Phase 1 system. All parameters are dynamic — nothing is hardcoded
// in the pipeline. Users answer questions to configure datasets, kinds, strategies,
// agents, and metric targets before each run.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Prompter handles interactive user input.
type Prompter struct {
	reader *bufio.Reader
	Auto   bool              // if true, use defaults without prompting
	Overrides map[string]string // pre-set answers for testing
}

func NewPrompter() *Prompter {
	return &Prompter{
		reader:    bufio.NewReader(os.Stdin),
		Overrides: make(map[string]string),
	}
}

// NewAutoPrompter returns a prompter that always uses defaults.
func NewAutoPrompter(overrides map[string]string) *Prompter {
	if overrides == nil {
		overrides = make(map[string]string)
	}
	return &Prompter{
		reader:    bufio.NewReader(strings.NewReader("")),
		Auto:      true,
		Overrides: overrides,
	}
}

// AskString prompts for a string value with a default.
func (p *Prompter) AskString(key, prompt, defaultVal string) string {
	if v, ok := p.Overrides[key]; ok {
		return v
	}
	if p.Auto {
		return defaultVal
	}
	fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	line, _ := p.reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// AskFloat prompts for a float value with a default.
func (p *Prompter) AskFloat(key, prompt string, defaultVal float64) float64 {
	if v, ok := p.Overrides[key]; ok {
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return f
		}
	}
	if p.Auto {
		return defaultVal
	}
	fmt.Printf("  %s [%.2f]: ", prompt, defaultVal)
	line, _ := p.reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	f, err := strconv.ParseFloat(line, 64)
	if err != nil {
		fmt.Printf("  Invalid number, using default %.2f\n", defaultVal)
		return defaultVal
	}
	return f
}

// AskInt prompts for an integer value with a default.
func (p *Prompter) AskInt(key, prompt string, defaultVal int) int {
	if v, ok := p.Overrides[key]; ok {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
	}
	if p.Auto {
		return defaultVal
	}
	fmt.Printf("  %s [%d]: ", prompt, defaultVal)
	line, _ := p.reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(line)
	if err != nil {
		fmt.Printf("  Invalid number, using default %d\n", defaultVal)
		return defaultVal
	}
	return i
}

// AskChoice prompts user to choose from a list. Returns the chosen string.
func (p *Prompter) AskChoice(key, prompt string, choices []string, defaultIdx int) string {
	if v, ok := p.Overrides[key]; ok {
		for _, c := range choices {
			if c == v {
				return v
			}
		}
	}
	if p.Auto {
		if defaultIdx >= 0 && defaultIdx < len(choices) {
			return choices[defaultIdx]
		}
		return choices[0]
	}
	fmt.Printf("  %s\n", prompt)
	for i, c := range choices {
		marker := "  "
		if i == defaultIdx {
			marker = "> "
		}
		fmt.Printf("    %s%d) %s\n", marker, i+1, c)
	}
	fmt.Printf("  Choice [%d]: ", defaultIdx+1)
	line, _ := p.reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return choices[defaultIdx]
	}
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 1 || idx > len(choices) {
		fmt.Printf("  Invalid choice, using default: %s\n", choices[defaultIdx])
		return choices[defaultIdx]
	}
	return choices[idx-1]
}

// AskMultiChoice prompts user to select multiple items (comma-separated indices).
func (p *Prompter) AskMultiChoice(key, prompt string, choices []string, defaultIdxs []int) []string {
	if v, ok := p.Overrides[key]; ok {
		return strings.Split(v, ",")
	}
	if p.Auto {
		result := make([]string, len(defaultIdxs))
		for i, idx := range defaultIdxs {
			if idx < len(choices) {
				result[i] = choices[idx]
			}
		}
		return result
	}
	fmt.Printf("  %s (comma-separated numbers)\n", prompt)
	for i, c := range choices {
		marker := "  "
		for _, d := range defaultIdxs {
			if i == d {
				marker = "* "
				break
			}
		}
		fmt.Printf("    %s%d) %s\n", marker, i+1, c)
	}
	defStrs := make([]string, len(defaultIdxs))
	for i, d := range defaultIdxs {
		defStrs[i] = strconv.Itoa(d + 1)
	}
	fmt.Printf("  Choices [%s]: ", strings.Join(defStrs, ","))
	line, _ := p.reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		result := make([]string, len(defaultIdxs))
		for i, idx := range defaultIdxs {
			result[i] = choices[idx]
		}
		return result
	}

	parts := strings.Split(line, ",")
	var result []string
	for _, part := range parts {
		idx, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || idx < 1 || idx > len(choices) {
			continue
		}
		result = append(result, choices[idx-1])
	}
	if len(result) == 0 {
		result = make([]string, len(defaultIdxs))
		for i, idx := range defaultIdxs {
			result[i] = choices[idx]
		}
	}
	return result
}

// AskYesNo asks a yes/no question.
func (p *Prompter) AskYesNo(key, prompt string, defaultYes bool) bool {
	if v, ok := p.Overrides[key]; ok {
		return v == "yes" || v == "y" || v == "true"
	}
	if p.Auto {
		return defaultYes
	}
	def := "Y/n"
	if !defaultYes {
		def = "y/N"
	}
	fmt.Printf("  %s [%s]: ", prompt, def)
	line, _ := p.reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

// Section prints a section header.
func (p *Prompter) Section(title string) {
	fmt.Printf("\n--- %s ---\n", title)
}

// Info prints an info message.
func (p *Prompter) Info(format string, args ...interface{}) {
	fmt.Printf("  [info] "+format+"\n", args...)
}
