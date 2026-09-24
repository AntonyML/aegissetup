//go:build !windows

package precheck

func printerApta() (bool, string) {
	return true, "solo Windows: no aplica fuera de Windows"
}
