// © Antony Monge López — Costa Rica — Céd. 604700548
package check

import (
	"strings"
	"testing"
)

func TestMSDASQL32RechazaMDYAunqueLaConexionAbra(t *testing.T) {
	// Bajo mdy, 25/9 falla y 5/9 se convierte en 9 de mayo silenciosamente.
	output := []byte(`{"connected":true,"language":"us_english","dateFormat":"mdy","planDate":"","ambiguousDate":"20260509","reverseDate":"20260905","compactDate":"20260925","isoDate":"20260925"}`)
	got := parseMSDASQL32Output(output)
	if !got.ConnectionOK {
		t.Fatal("el problema de fechas no debe convertir una conexión abierta en fallo de login")
	}
	if got.DatesOK {
		t.Fatal("el check aceptó mdy y una fecha ambigua interpretada con mes/día")
	}
	if !strings.Contains(got.DatesInfo, "mdy") {
		t.Fatalf("el diagnóstico no identifica el formato real: %s", got.DatesInfo)
	}
}

func TestMSDASQL32AceptaDMYConFechasExactas(t *testing.T) {
	output := []byte(`{"connected":true,"language":"Español","dateFormat":"dmy","planDate":"20260925","ambiguousDate":"20260905","reverseDate":"20260509","compactDate":"20260925","isoDate":"20260925"}`)
	got := parseMSDASQL32Output(output)
	if !got.ConnectionOK || !got.DatesOK {
		t.Fatalf("la sesión dmy rechazó fechas correctas: %s; %s", got.ConnectionInfo, got.DatesInfo)
	}
	if !strings.Contains(got.DatesInfo, "09/05/2026=20260509") {
		t.Fatalf("el diagnóstico no muestra la segunda fecha ambigua: %s", got.DatesInfo)
	}
}

func TestMSDASQL32NoAceptaSoloElFormatoSinComprobarLaConversion(t *testing.T) {
	output := []byte(`{"connected":true,"language":"Español","dateFormat":"dmy","planDate":"20260925","ambiguousDate":"20260509","reverseDate":"20260509","compactDate":"20260925","isoDate":"20260925"}`)
	got := parseMSDASQL32Output(output)
	if got.DatesOK {
		t.Fatal("DATEFORMAT=dmy no alcanza si 5/9/2026 se convirtió en 9 de mayo")
	}
}

func TestMSDASQL32ConservaLoginCuandoFallaLaConsultaDeFechas(t *testing.T) {
	got := parseMSDASQL32Output([]byte(`{"connected":true,"errorCode":242}`))
	if !got.ConnectionOK || got.DatesOK {
		t.Fatalf("el error de conversión debe fallar solo fechas: %+v", got)
	}
	if !strings.Contains(got.DatesInfo, "242") {
		t.Fatalf("falta el código SQL en el diagnóstico: %s", got.DatesInfo)
	}
}

func TestMSDASQL32NoDaVerdeAnteRespuestaIncompletaOInvalida(t *testing.T) {
	for _, output := range []string{"OK", `{}`, `{"connected":true}`, `{"connected":"true"}`} {
		t.Run(output, func(t *testing.T) {
			got := parseMSDASQL32Output([]byte(output))
			if got.DatesOK {
				t.Fatal("la sonda aceptó fechas sin haber recibido sus conversiones")
			}
		})
	}
}

func TestMSDASQL32NoReproduceErroresCrudosDelSubproceso(t *testing.T) {
	output := "FAIL: contenido que no debe salir del subproceso"
	got := parseMSDASQL32Output([]byte(output))
	if strings.Contains(got.ConnectionInfo, "contenido") || strings.Contains(got.DatesInfo, "contenido") {
		t.Fatal("el error inválido del subproceso se reprodujo en el diagnóstico")
	}
	if got.ConnectionOK || got.DatesOK {
		t.Fatal("una respuesta inválida no puede dar verde")
	}
}
