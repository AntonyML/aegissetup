// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"os"
	"strings"
)

// envNoElevar marca al proceso que ya es el relanzado elevado.
//
// Sin esta marca, un proceso elevado que por lo que sea no se viera elevado
// reintentaría elevarse: el operador vería una sucesión de carteles de UAC sin
// poder llegar nunca al trabajo.
const envNoElevar = "AEGIS_NO_ELEVAR"

// IsAdmin dice si este proceso ya tiene permisos de administrador.
//
// Es la única fuente de la respuesta: el checklist, el CLI y el TUI tienen que
// coincidir. Si cada uno preguntara lo suyo, una PC podría decir "falta permisos"
// en una pantalla y escribir sin problema en la siguiente.
func IsAdmin() bool { return isAdminOS() }

// SkipElevation dice si esta corrida no debe volver a intentar elevarse.
func SkipElevation() bool { return os.Getenv(envNoElevar) != "" }

// NeedsElevation decide si hay que relanzar como administrador. Son dos banderas y
// una sola respuesta: tenerla acá la hace probable sin un UAC de por medio.
func NeedsElevation(admin, skip bool) bool { return !admin && !skip }

// ElevatedArgs es lo que se le pasa al proceso elevado: los mismos argumentos sin
// el nombre del programa, porque allá el programa es el ejecutable.
func ElevatedArgs(args []string) []string {
	if len(args) <= 1 {
		return nil
	}
	return append([]string(nil), args[1:]...)
}

// Elevate relanza este programa con permisos de administrador y ESPERA a que
// termine, devolviendo su código de salida.
//
// Esperar no es un detalle: un comando que encadena pasos (instalar y después
// verificar) seguiría con el siguiente antes de que el primero terminara, y el
// operador vería un error que en realidad era una carrera.
//
// El error es solo si no se pudo lanzar. Lo más común es que el operador haya
// cancelado el pedido de permisos, y eso hay que decirlo tal cual.
func Elevate(args []string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	return launchElevated(exe, ElevatedArgs(args), true)
}

// RelaunchElevated lanza el proceso elevado SIN esperarlo. Es lo que necesita el
// TUI: la pantalla se cierra enseguida y la otra ventana sigue trabajando.
func RelaunchElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	_, err = launchElevated(exe, ElevatedArgs(args), false)
	return err
}

// quoteArgs arma la línea de parámetros como la espera CommandLineToArgvW.
//
// Un argumento con espacios sin comillar llega partido en dos al proceso elevado:
// --config "C:\Program Files\...\config.json" se convertiría en dos argumentos y el
// comando levantaría un config que no existe. Las barras antes de una comilla se
// duplican porque son el carácter de escape del formato.
func quoteArgs(args []string) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		if a != "" && !strings.ContainsAny(a, " \t\"") {
			b.WriteString(a)
			continue
		}
		b.WriteByte('"')
		barras := 0
		for _, r := range a {
			switch r {
			case '\\':
				barras++
				b.WriteRune(r)
			case '"':
				// Las barras que venían antes se duplican para que no escapen esta comilla.
				b.WriteString(strings.Repeat(`\`, barras))
				b.WriteString(`\"`)
				barras = 0
			default:
				barras = 0
				b.WriteRune(r)
			}
		}
		// Las barras del final se duplican: si no, escaparían la comilla de cierre.
		b.WriteString(strings.Repeat(`\`, barras))
		b.WriteByte('"')
	}
	return b.String()
}
