package configgenerator

import (
	"bytes"
	_ "embed"
	"io"
	"log/slog"
	"strings"
	"text/template"

	"github.com/scality/go-errors"
	"github.com/scality/virtualip-manager/pkg/domain"
	"github.com/scality/virtualip-manager/pkg/service"
	"go.yaml.in/yaml/v3"
)

type Keepalived struct {
	logger          *slog.Logger
	interfaceGetter service.InterfaceGetter
	nodeIP          string
	nodeName        string
}

func NewKeepalived(
	logger *slog.Logger,
	interfaceGetter service.InterfaceGetter,
	nodeIP string,
	nodeName string,
) *Keepalived {
	l := logger.With(
		slog.String("infrastructure", "configgenerator"),
		slog.String("implementation", "keepalived"),
	)
	return &Keepalived{
		logger:          l,
		interfaceGetter: interfaceGetter,
		nodeIP:          nodeIP,
		nodeName:        nodeName,
	}
}

var _ service.ConfigGenerator = &Keepalived{}

//go:embed keepalived.tmpl
var templateContent string

// templateData is the root context passed to the keepalived template. It
// embeds the parsed config so .Addresses/.Healthcheck still resolve, and adds
// the node identity so the template no longer reads the environment directly.
type templateData struct {
	*domain.VirtualIPConfig
	NodeIP      string
	NodeName    string
	NodeIPToken string
}

// GenerateConfiguration generates the output configuration from the input data.
func (k *Keepalived) GenerateConfiguration(inputData *domain.VirtualIPConfig) (string, error) {
	tmpl, err := template.New("keepalived").
		Funcs(templateFuncs(k.interfaceGetter)).
		Parse(templateContent)
	if err != nil {
		return "", errors.Wrap(domain.ErrTemplating,
			errors.WithDetail("failed to parse template"),
			errors.WithProperty("template", templateContent),
			errors.CausedBy(err),
		)
	}

	data := templateData{
		VirtualIPConfig: inputData,
		NodeIP:          k.nodeIP,
		NodeName:        k.nodeName,
		NodeIPToken:     domain.NODE_IP_TOKEN,
	}

	outputData := strings.Builder{}
	err = tmpl.Execute(&outputData, data)
	if err != nil {
		return "", errors.Wrap(domain.ErrTemplating,
			errors.WithDetail("failed to execute template"),
			errors.WithProperty("template", templateContent),
			errors.CausedBy(err),
		)
	}

	return outputData.String(), nil
}

func (k *Keepalived) ParseInputData(inputData []byte) (*domain.VirtualIPConfig, error) {
	var parsedInputData *domain.VirtualIPConfig
	decoder := yaml.NewDecoder(bytes.NewReader(inputData))
	decoder.KnownFields(true)
	// yaml/v3 returns io.EOF for empty input, where v2's UnmarshalStrict
	// returned a nil error and left the pointer nil. Treat EOF the same so
	// the empty-input check below still owns that case.
	err := decoder.Decode(&parsedInputData)
	if err != nil && err != io.EOF {
		return nil, errors.Wrap(domain.ErrInputFileParsing,
			errors.WithDetail("failed to parse input data"),
			errors.WithProperty("inputData", string(inputData)),
			errors.CausedBy(err),
		)
	}

	// For an empty file, null, ~, or a comment-only file, yaml
	// leaves the pointer nil and returns no error
	if parsedInputData == nil {
		return nil, errors.Wrap(domain.ErrMissingInputParameter,
			errors.WithDetail("empty input data"),
		)
	}

	// Normalize before validating so an empty healthcheck key reads as absent
	// rather than as an invalid URL.
	parsedInputData.CleanHealthchecks()

	err = parsedInputData.Validate()
	if err != nil {
		return nil, errors.Wrap(err,
			errors.WithDetail("failed to validate input data"),
			errors.WithProperty("inputData", string(inputData)),
		)
	}

	return parsedInputData, nil
}

// templateFuncs returns the custom functions made available to the keepalived
// template.
func templateFuncs(interfaceGetter service.InterfaceGetter) template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"replace": func(s *string, old, replacement string) string {
			if s == nil {
				return ""
			}
			return strings.ReplaceAll(*s, old, replacement)
		},
		"getInterfaceFromIP": interfaceGetter.GetInterfaceFromIP,
	}
}
