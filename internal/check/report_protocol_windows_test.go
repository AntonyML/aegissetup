//go:build windows

// © Antony Monge López — Costa Rica — Céd. 604700548

package check

import "testing"

func TestParseReportHostTraceConservaEtapaYDetalles(t *testing.T) {
	trace := parseReportHostTrace([]byte(`{"success":true,"stage":"read_records","errorCode":null,"hresult":null,"crystalError":null,"elapsedMs":123,"details":{"tableCount":5,"subreportCount":0}}`))
	if len(trace) != 1 || !trace[0].Success || trace[0].Stage != "read_records" {
		t.Fatalf("trace = %+v", trace)
	}
	if got := trace[0].Details["tableCount"].(float64); got != 5 {
		t.Fatalf("tableCount = %v, quiero 5", got)
	}
}

func TestFormatReportHostErrorNoPierdeHRESULT(t *testing.T) {
	got := formatReportHostError("Rpt_Caja_Chica.rpt", reportHostResponse{
		Stage:        "set_report_source",
		ErrorCode:    "automation_error",
		HRESULT:      "0x80010108",
		CrystalError: "The object invoked has disconnected from its clients",
	}, nil)
	for _, want := range []string{"stage=set_report_source", "errorCode=automation_error", "hresult=0x80010108", "crystalError="} {
		if !contains(got, want) {
			t.Fatalf("error = %q; falta %q", got, want)
		}
	}
}

func TestReportHostTimeoutStageInfereLaSiguienteEtapa(t *testing.T) {
	trace := []reportHostResponse{{Stage: "com_application", Success: true}}
	if got := reportHostTimeoutStage(trace); got != "open_report" {
		t.Fatalf("stage = %q, quiero open_report", got)
	}
	if got := reportHostTimeoutStage([]reportHostResponse{{Stage: "table_logon_complete", Success: true}}); got != "parameters" {
		t.Fatalf("stage = %q, quiero parameters", got)
	}
	if got := reportHostTimeoutStage([]reportHostResponse{{Stage: "discard_saved_data", Success: true}}); got != "read_records" {
		t.Fatalf("stage = %q, quiero read_records", got)
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
