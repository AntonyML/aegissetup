// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// creaImagenPrueba genera una imagen RGBA simple para tests.
func creaImagenPrueba(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 0, G: 120, B: 215, A: 255}}, image.Point{}, draw.Src)
	return img
}

func TestGeneratePaddedJPEGExactLengths(t *testing.T) {
	src := creaImagenPrueba(300, 150)

	casos := []struct {
		nombre string
		w, h   int
		tamano int
	}{
		{"LogoFormulario", logoFormW, logoFormH, logoFormBytes},
		{"Fondo", logoBgW, logoBgH, logoBgBytes},
		{"Splash", logoSplashW, logoSplashH, logoSplashBytes},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			b, err := GeneratePaddedJPEG(src, c.w, c.h, c.tamano)
			if err != nil {
				t.Fatalf("GeneratePaddedJPEG falló: %v", err)
			}
			if len(b) != c.tamano {
				t.Fatalf("longitud = %d, quiero exactamente %d", len(b), c.tamano)
			}

			// Debe ser un JPEG válido decodificable por el parser estándar
			decoded, err := jpeg.Decode(bytes.NewReader(b))
			if err != nil {
				t.Fatalf("jpeg.Decode falló en el JPEG con padding: %v", err)
			}
			bounds := decoded.Bounds()
			if bounds.Dx() != c.w || bounds.Dy() != c.h {
				t.Errorf("dimensiones decodificadas = %dx%d, quiero %dx%d",
					bounds.Dx(), bounds.Dy(), c.w, c.h)
			}
		})
	}
}

func TestReplaceImagesAndHideLabelsSintetico(t *testing.T) {
	// Armar un buffer sintético que simula un formulario de VB6:
	// Label16 (Caption="******* CAPROBA 1981", Coords), seguido de Image4 con cabecera y 9496 bytes JPEG.
	var buf bytes.Buffer

	// 1. Relleno previo
	buf.WriteString("PREFIX_DATA...")

	// 2. Control Label16:
	// Caption prop: 0x01, 0x01, len(20), 0x00
	caption := []byte("******* CAPROBA 1981")
	buf.Write([]byte{0x01, 0x01, byte(len(caption)), 0x00})
	buf.Write(caption)
	// Propiedad de coordenadas 0x05: Left(2), Top(2), Width(2), Height(2)
	// Left=495 (EF 01), Top=555 (2B 02), Width=765 (FD 02), Height=420 (A4 01)
	buf.WriteByte(0x05)
	buf.Write([]byte{0xEF, 0x01, 0x2B, 0x02, 0xFD, 0x02, 0xA4, 0x01})

	// 3. Relleno intermedio
	buf.WriteString("...MID...")

	// 4. Cabecera OLE Picture Form: sigFormLogo
	buf.Write(sigFormLogo)
	// Cuerpo del JPEG simulado (9496 - 2 bytes de FF D8 que ya están en la firma)
	fakeOrigJPEG := make([]byte, logoFormBytes-2)
	for i := range fakeOrigJPEG {
		fakeOrigJPEG[i] = 0xAA
	}
	buf.Write(fakeOrigJPEG)

	// 5. Relleno posterior
	buf.WriteString("...POST_DATA...")

	originalData := buf.Bytes()
	origLen := len(originalData)

	// Parchear
	src := creaImagenPrueba(200, 200)
	patched, rep, err := PatchLogos(originalData, src)
	if err != nil {
		t.Fatalf("PatchLogos falló: %v", err)
	}

	if len(patched) != origLen {
		t.Fatalf("PatchLogos alteró el tamaño total: antes=%d, después=%d", origLen, len(patched))
	}
	if rep.FormLogos != 1 {
		t.Errorf("rep.FormLogos = %d, quiero 1", rep.FormLogos)
	}
	if rep.Labels != 1 {
		t.Errorf("rep.Labels = %d, quiero 1", rep.Labels)
	}

	// Verificar que el texto del Caption ahora son puros espacios
	idx := bytes.Index(patched, []byte("                    "))
	if idx < 0 {
		t.Errorf("no se encontraron los espacios del caption neutralizado")
	}

	// Verificar que las coordenadas en 0x05 ahora son Width=0, Height=0, Left=-30000
	coordIdx := idx + len(caption)
	if patched[coordIdx] != 0x05 {
		t.Fatalf("byte en coordIdx = 0x%02X, esperaba 0x05", patched[coordIdx])
	}
	// Left = D0 8A (-30000)
	if patched[coordIdx+1] != 0xD0 || patched[coordIdx+2] != 0x8A {
		t.Errorf("Left no es -30000: %X %X", patched[coordIdx+1], patched[coordIdx+2])
	}
	// Width = 00 00
	if patched[coordIdx+5] != 0x00 || patched[coordIdx+6] != 0x00 {
		t.Errorf("Width no es 0: %X %X", patched[coordIdx+5], patched[coordIdx+6])
	}
	// Height = 00 00
	if patched[coordIdx+7] != 0x00 || patched[coordIdx+8] != 0x00 {
		t.Errorf("Height no es 0: %X %X", patched[coordIdx+7], patched[coordIdx+8])
	}
}

func TestGenerateReportBMPSintetico(t *testing.T) {
	src := creaImagenPrueba(500, 250)

	bmpBytes, err := GenerateReportBMP(src)
	if err != nil {
		t.Fatalf("GenerateReportBMP falló: %v", err)
	}

	if len(bmpBytes) != reportBMPBytes {
		t.Fatalf("longitud BMP = %d, quiero exactamente %d", len(bmpBytes), reportBMPBytes)
	}

	// Verificar cabecera BITMAPFILEHEADER
	if bmpBytes[0] != 0x42 || bmpBytes[1] != 0x4D {
		t.Errorf("firma no es 'BM': %c %c", bmpBytes[0], bmpBytes[1])
	}

	// Simular un directorio de reportes con un .rpt falso que contiene la firma
	dir := t.TempDir()
	rptFalso := filepath.Join(dir, "TestReporte.rpt")

	var buf bytes.Buffer
	buf.WriteString("HEADER_OLE_D0CF11E0...")
	buf.Write(sigReportBMP)
	// Completar el tamaño del BMP simulado original
	buf.Write(make([]byte, reportBMPBytes-len(sigReportBMP)))
	buf.WriteString("...TAIL_SECTOR...")

	if err := os.WriteFile(rptFalso, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	n, err := PatchReportsLogos(dir, src, func(string) {})
	if err != nil {
		t.Fatalf("PatchReportsLogos falló: %v", err)
	}
	if n != 1 {
		t.Errorf("reportes parchados = %d, quiero 1", n)
	}

	// Verificar que el archivo resultante conservó exactamente su tamaño
	info, err := os.Stat(rptFalso)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(buf.Len()) {
		t.Errorf("tamaño final = %d, quiero %d", info.Size(), buf.Len())
	}
}

func TestPatchAppAndReportsMissingFiles(t *testing.T) {
	// 1. Directorio vacío sin Fotos/Principal.jpg debe informar y devolver error descriptivo
	emptyDir := t.TempDir()
	var logs []string
	emit := func(s string) { logs = append(logs, s) }

	err := PatchAppAndReports(emptyDir, emit)
	if err == nil {
		t.Fatalf("se esperaba error por falta de Fotos/Principal.jpg, obtuve nil")
	}
	if len(logs) == 0 {
		t.Errorf("se esperaba mensaje de log avisando la falta del archivo")
	}

	// 2. Con Fotos/Principal.jpg pero sin binarios ni reportes no debe caerse ni entrar en pánico
	fotosDir := filepath.Join(emptyDir, "Fotos")
	if err := os.MkdirAll(fotosDir, 0755); err != nil {
		t.Fatal(err)
	}
	dummyImg, err := GeneratePaddedJPEG(creaImagenPrueba(50, 50), 50, 50, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fotosDir, "Principal.jpg"), dummyImg, 0644); err != nil {
		t.Fatal(err)
	}

	logs = nil
	err = PatchAppAndReports(emptyDir, emit)
	if err != nil {
		t.Fatalf("no debió fallar por no tener binarios opcionales: %v", err)
	}
	// Debe haber avisado que no se encontró el ejecutable principal y los reportes
	encontradoAviso := false
	for _, l := range logs {
		if strings.Contains(l, "Aviso:") {
			encontradoAviso = true
			break
		}
	}
	if !encontradoAviso {
		t.Errorf("se esperaba al menos un aviso informativo, logs: %v", logs)
	}
}

