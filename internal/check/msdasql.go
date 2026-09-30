// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"encoding/json"
	"fmt"
	"strings"
)

// msdasqlProbe separa login y fechas: Crystal sigue usando la cadena del EXE
// cuando el login abrió, aunque el formato de fechas necesite reparación.
type msdasqlProbe struct {
	ConnectionOK   bool
	ConnectionInfo string
	DatesOK        bool
	DatesInfo      string
}

type msdasqlResponse struct {
	Connected     bool   `json:"connected"`
	Language      string `json:"language"`
	DateFormat    string `json:"dateFormat"`
	PlanDate      string `json:"planDate"`
	AmbiguousDate string `json:"ambiguousDate"`
	ReverseDate   string `json:"reverseDate"`
	CompactDate   string `json:"compactDate"`
	ISODate       string `json:"isoDate"`
	ErrorCode     int    `json:"errorCode"`
}

func parseMSDASQL32Output(output []byte) msdasqlProbe {
	var response msdasqlResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return msdasqlProbe{
			ConnectionInfo: "respuesta inválida de la prueba MSDASQL 32-bit",
			DatesInfo:      "fechas no verificadas: no se recibió una respuesta válida",
		}
	}
	if !response.Connected {
		return msdasqlProbe{
			ConnectionInfo: fmt.Sprintf("no se pudo abrir la conexión MSDASQL 32-bit (error SQL %d)", response.ErrorCode),
			DatesInfo:      "fechas no verificadas: la conexión no abrió",
		}
	}
	result := msdasqlProbe{
		ConnectionOK:   true,
		ConnectionInfo: "conexión 32-bit MSDASQL verificada",
	}
	if response.ErrorCode != 0 {
		result.DatesInfo = fmt.Sprintf("no se pudieron consultar las fechas de la sesión (error SQL %d)", response.ErrorCode)
		return result
	}
	result.DatesOK = response.DateFormat == "dmy" &&
		response.PlanDate == "20260925" && response.AmbiguousDate == "20260905" &&
		response.ReverseDate == "20260509" && response.CompactDate == "20260925" && response.ISODate == "20260925"
	result.DatesInfo = fmt.Sprintf("idioma=%s DATEFORMAT=%s; 25/9/2026=%s; 5/9/2026=%s; 09/05/2026=%s; YYYYMMDD=%s; ISO=%s",
		response.Language, response.DateFormat, dateProbeValue(response.PlanDate), dateProbeValue(response.AmbiguousDate),
		dateProbeValue(response.ReverseDate), dateProbeValue(response.CompactDate), dateProbeValue(response.ISODate))
	if !result.DatesOK {
		result.DatesInfo = "la sesión no interpreta correctamente día/mes/año: " + result.DatesInfo
	}
	return result
}

func dateProbeValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "inválida"
	}
	return value
}
