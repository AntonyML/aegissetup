//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aegis-setup/internal/config"
	"aegis-setup/internal/setup"
)

// openReportSample usa el mismo host x86 que la vista visual, pero en modo carga:
// el watchdog termina solo esta fase y nunca alcanza una ventana interactiva.
func openReportSample(ctx context.Context, repDir string, cfg config.Config, appPass, appConnStr string) (bool, string) {
	path, err := reportPath(repDir)
	if err != nil {
		return false, reportUnderCheck + ": " + err.Error()
	}
	return openReportHost(ctx, path, cfg, appPass, appConnStr, "check", "")
}

// OpenReportVisual abre Rpt_Caja_Chica en el host x86 y conserva el proceso hasta
// que el usuario cierre la ventana. No se le aplica el timeout del check.
func OpenReportVisual(ctx context.Context, repDir string, cfg config.Config, appPass, appConnStr string) (bool, string) {
	path, err := reportPath(repDir)
	if err != nil {
		return false, reportUnderCheck + ": " + err.Error()
	}
	return openReportHost(ctx, path, cfg, appPass, appConnStr, "view", "")
}

// ExportReportPDF conserva Crystal solo para abrir, autenticar y leer el reporte.
// La generación final la hace el puente moderno dentro de ReportHost, sin usar el
// exportador PDF nativo de Crystal 8.5.
func ExportReportPDF(ctx context.Context, repDir string, cfg config.Config, appPass, appConnStr, outputPath string) (bool, string) {
	path, err := reportPath(repDir)
	if err != nil {
		return false, reportUnderCheck + ": " + err.Error()
	}
	return openReportHost(ctx, path, cfg, appPass, appConnStr, "export-pdf", outputPath)
}

type reportHostResponse struct {
	Success      bool                   `json:"success"`
	Stage        string                 `json:"stage"`
	ErrorCode    string                 `json:"errorCode"`
	HRESULT      string                 `json:"hresult"`
	CrystalError string                 `json:"crystalError"`
	ElapsedMs    int64                  `json:"elapsedMs"`
	Details      map[string]interface{} `json:"details"`
}

func openReportHost(ctx context.Context, reportPath string, cfg config.Config, appPass, appConnStr, mode, outputPath string) (bool, string) {
	reportName := filepath.Base(reportPath)
	host, err := reportHostPath()
	if err != nil {
		return false, reportName + ": stage=process_start errorCode=host_missing crystalError=" + err.Error()
	}
	user := strings.TrimSpace(cfg.SQLUser)
	if user == "" {
		user = "sidc"
	}
	password := strings.TrimSpace(appPass)
	if appConnStr != "" {
		if embeddedUser, embeddedPassword := setup.ExeConnectionCredentials(appConnStr); embeddedUser != "" {
			user = embeddedUser
			password = embeddedPassword
		}
	}
	if !cfg.UseWinAuth && password == "" {
		password = setup.DSNPassword(cfg.DsnName)
	}
	dsn := strings.TrimSpace(cfg.DsnName)
	if dsn == "" {
		dsn = "SIDC_SQL"
	}
	database := strings.TrimSpace(cfg.Database)
	if database == "" {
		database = "SIDC"
	}

	args := []string{"--mode", mode, "--report", reportPath, "--dsn", dsn, "--database", database, "--user", user}
	if outputPath != "" {
		args = append(args, "--output", outputPath)
	}
	runCtx := ctx
	cancel := func() {}
	if mode == "check" {
		runCtx, cancel = context.WithTimeout(ctx, 15*time.Second)
	} else if mode == "export-pdf" {
		runCtx, cancel = context.WithTimeout(ctx, 90*time.Second)
	}
	defer cancel()
	cmd := exec.CommandContext(runCtx, host, args...)
	cmd.Env = checkCommandEnv(map[string]string{
		"AEGIS_REPORT_PASSWORD": password,
		"AEGIS_REPORT_WIN_AUTH": boolEnv(cfg.UseWinAuth),
	})
	started := time.Now()
	out, runErr := cmd.CombinedOutput()
	trace := parseReportHostTrace(out)
	if runCtx.Err() == context.DeadlineExceeded {
		return false, reportName + ": stage=" + reportHostTimeoutStage(trace) + " errorCode=timeout elapsedMs=" + strconv.FormatInt(time.Since(started).Milliseconds(), 10)
	}
	if len(trace) == 0 {
		if runErr == nil {
			return false, reportName + ": stage=process_exit errorCode=protocol_invalid crystalError=ReportHost no devolvió JSON"
		}
		return false, reportName + ": stage=process_start errorCode=host_process crystalError=" + strings.TrimSpace(runErr.Error())
	}
	last := trace[len(trace)-1]
	if !last.Success || runErr != nil {
		if last.ErrorCode == "" && runErr != nil {
			last.ErrorCode = "host_process"
		}
		return false, formatReportHostError(reportName, last, runErr)
	}
	if mode == "view" {
		return true, reportUnderCheck + ": viewer_ready y cierre limpio; COM liberado por ReportHost"
	}
	if mode == "export-pdf" {
		return true, reportName + ": PDF generado; stage=pdf_ready elapsedMs=" + strconv.FormatInt(last.ElapsedMs, 10)
	}
	tables := ""
	for _, event := range trace {
		if event.Stage != "read_records" || event.Details == nil {
			continue
		}
		if value, ok := event.Details["tableCount"].(float64); ok {
			tables = strconv.Itoa(int(value))
		}
	}
	return true, reportName + " abierta por Crystal 8; login aplicado a " + tables + " tablas; stage=process_exit elapsedMs=" + strconv.FormatInt(last.ElapsedMs, 10)
}

func reportHostPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("AEGIS_REPORT_HOST")); configured != "" {
		if _, err := os.Stat(configured); err != nil {
			return "", err
		}
		return configured, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	host := filepath.Join(filepath.Dir(exe), "Aegis.ReportHost.exe")
	if _, err := os.Stat(host); err != nil {
		return "", err
	}
	return host, nil
}

func parseReportHostTrace(output []byte) []reportHostResponse {
	var trace []reportHostResponse
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event reportHostResponse
		if json.Unmarshal([]byte(line), &event) == nil {
			trace = append(trace, event)
		}
	}
	return trace
}

func formatReportHostError(reportName string, event reportHostResponse, runErr error) string {
	parts := []string{reportName + ": stage=" + event.Stage}
	if event.ErrorCode != "" {
		parts = append(parts, "errorCode="+event.ErrorCode)
	}
	if event.HRESULT != "" {
		parts = append(parts, "hresult="+event.HRESULT)
	}
	if event.CrystalError != "" {
		parts = append(parts, "crystalError="+event.CrystalError)
	}
	if event.Details != nil {
		if errorType, ok := event.Details["errorType"].(string); ok && errorType != "" {
			parts = append(parts, "errorType="+errorType)
		}
		if stackTrace, ok := event.Details["stackTrace"].(string); ok && stackTrace != "" {
			parts = append(parts, "stackTrace="+stackTrace)
		}
	}
	if runErr != nil && event.CrystalError == "" {
		parts = append(parts, "processError="+runErr.Error())
	}
	return strings.Join(parts, " ")
}

func reportHostTimeoutStage(trace []reportHostResponse) string {
	if len(trace) == 0 {
		return "process_start"
	}
	switch trace[len(trace)-1].Stage {
	case "process_start":
		return "com_application"
	case "com_application":
		return "open_report"
	case "open_report":
		return "database_logon"
	case "database_logon", "table_logon":
		return "table_logon"
	case "table_logon_complete":
		return "parameters"
	case "parameters":
		return "discard_saved_data"
	case "discard_saved_data":
		return "read_records"
	case "read_records":
		return "viewer_create"
	case "viewer_create":
		return "set_report_source"
	case "set_report_source":
		return "view_report"
	case "export_prepare", "modern_data_query":
		return "modern_html"
	case "modern_html":
		return "pdf_render"
	case "pdf_render":
		return "pdf_ready"
	default:
		return trace[len(trace)-1].Stage
	}
}

func boolEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func checkCommandEnv(values map[string]string) []string {
	env := os.Environ()
	for key, value := range values {
		prefix := key + "="
		filtered := make([]string, 0, len(env))
		for _, item := range env {
			if !strings.HasPrefix(item, prefix) {
				filtered = append(filtered, item)
			}
		}
		env = append(filtered, prefix+value)
	}
	return env
}
