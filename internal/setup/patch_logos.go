// © Antony Monge López — Costa Rica — Céd. 604700548
package setup

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// Constantes de dimensiones y tamaños binarios de las imágenes incrustadas en el
// ejecutable de VB6 ("Sistema Integrado de Controles y Presupuesto.exe").
const (
	logoFormW, logoFormH, logoFormBytes       = 237, 237, 9496
	logoBgW, logoBgH, logoBgBytes             = 1183, 1200, 73212
	logoSplashW, logoSplashH, logoSplashBytes = 1500, 1125, 148262

	// Reportes Crystal Reports (.rpt): BMP 32-bit sin comprimir
	reportBMPW, reportBMPH, reportBMPBytes = 1183, 1200, 5678490
)

// Cabeceras OLE Picture en VB6: prefijo 'lt\0\0' seguido del largo uint32 little-endian
// y los primeros bytes del JPEG (FF D8).
var (
	sigFormLogo = []byte{0x6C, 0x74, 0x00, 0x00, 0x18, 0x25, 0x00, 0x00, 0xFF, 0xD8}
	sigBgLogo   = []byte{0x6C, 0x74, 0x00, 0x00, 0xFC, 0x1D, 0x01, 0x00, 0xFF, 0xD8}
	sigSplash   = []byte{0x6C, 0x74, 0x00, 0x00, 0x26, 0x43, 0x02, 0x00, 0xFF, 0xD8}

	// Cabecera BITMAPFILEHEADER + BITMAPINFOHEADER (30 bytes de firma fija) del BMP de 5.678.490 bytes en .rpt
	sigReportBMP = []byte{
		0x42, 0x4D, 0x9A, 0xA5, 0x56, 0x00, 0x00, 0x00, 0x00, 0x00, 0x36, 0x00, 0x00, 0x00,
		0x28, 0x00, 0x00, 0x00, 0x9F, 0x04, 0x00, 0x00, 0xB0, 0x04, 0x00, 0x00, 0x01, 0x00, 0x20, 0x00,
	}
)

// PatchReport resume los reemplazos aplicados en el binario.
type PatchReport struct {
	FormLogos   int
	Backgrounds int
	Splashes    int
	Labels      int
}

func (r PatchReport) Total() int {
	return r.FormLogos + r.Backgrounds + r.Splashes + r.Labels
}

// GeneratePaddedJPEG genera un slice de bytes JPEG válido de longitud exacta targetLen,
// escalando src con fondo blanco y agregando un marcador de comentario (COM, 0xFF 0xFE)
// con el relleno de bytes necesario.
func GeneratePaddedJPEG(src image.Image, targetW, targetH, targetLen int) ([]byte, error) {
	if src == nil {
		return nil, fmt.Errorf("imagen origen nula")
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("dimensiones origen inválidas (%dx%d)", srcW, srcH)
	}

	scaleW := float64(targetW) / float64(srcW)
	scaleH := float64(targetH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	scaledW := int(float64(srcW) * scale)
	scaledH := int(float64(srcH) * scale)
	offsetX := (targetW - scaledW) / 2
	offsetY := (targetH - scaledH) / 2

	destRect := image.Rect(offsetX, offsetY, offsetX+scaledW, offsetY+scaledH)
	draw.CatmullRom.Scale(dst, destRect, src, bounds, draw.Over, nil)

	var rawJPEG []byte
	for q := 85; q >= 10; q -= 5 {
		var buf bytes.Buffer
		opts := &jpeg.Options{Quality: q}
		if err := jpeg.Encode(&buf, dst, opts); err != nil {
			return nil, fmt.Errorf("error al codificar jpeg: %w", err)
		}
		if buf.Len() <= targetLen-10 {
			rawJPEG = buf.Bytes()
			break
		}
	}

	if rawJPEG == nil || len(rawJPEG) > targetLen-4 {
		return nil, fmt.Errorf("la imagen comprimida excede el límite (%d > %d)", len(rawJPEG), targetLen-4)
	}

	diff := targetLen - len(rawJPEG)
	payloadLen := diff - 4
	comLenField := uint16(payloadLen + 2)

	result := make([]byte, targetLen)
	copy(result[0:2], rawJPEG[0:2])
	result[2] = 0xFF
	result[3] = 0xFE
	result[4] = byte(comLenField >> 8)
	result[5] = byte(comLenField & 0xFF)
	copy(result[6+payloadLen:], rawJPEG[2:])

	return result, nil
}

// GenerateReportBMP genera un BMP de 32 bpp sin comprimir de exactamente 5.678.490 bytes (1183x1200),
// escalando src con fondo blanco.
func GenerateReportBMP(src image.Image) ([]byte, error) {
	if src == nil {
		return nil, fmt.Errorf("imagen origen nula")
	}

	dst := image.NewRGBA(image.Rect(0, 0, reportBMPW, reportBMPH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("dimensiones origen inválidas (%dx%d)", srcW, srcH)
	}

	scaleW := float64(reportBMPW) / float64(srcW)
	scaleH := float64(reportBMPH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	scaledW := int(float64(srcW) * scale)
	scaledH := int(float64(srcH) * scale)
	offsetX := (reportBMPW - scaledW) / 2
	offsetY := (reportBMPH - scaledH) / 2

	destRect := image.Rect(offsetX, offsetY, offsetX+scaledW, offsetY+scaledH)
	draw.CatmullRom.Scale(dst, destRect, src, bounds, draw.Over, nil)

	buf := make([]byte, reportBMPBytes)

	// BITMAPFILEHEADER (14 bytes)
	buf[0] = 0x42 // 'B'
	buf[1] = 0x4D // 'M'
	binary.LittleEndian.PutUint32(buf[2:6], uint32(reportBMPBytes))
	binary.LittleEndian.PutUint32(buf[10:14], 54) // Offset a datos

	// BITMAPINFOHEADER (40 bytes)
	binary.LittleEndian.PutUint32(buf[14:18], 40)
	binary.LittleEndian.PutUint32(buf[18:22], uint32(reportBMPW))
	binary.LittleEndian.PutUint32(buf[22:26], uint32(reportBMPH))
	binary.LittleEndian.PutUint16(buf[26:28], 1)               // Planes
	binary.LittleEndian.PutUint16(buf[28:30], 32)              // 32 bpp
	binary.LittleEndian.PutUint32(buf[38:42], 3779)            // XpixelsPerM (96 DPI)
	binary.LittleEndian.PutUint32(buf[42:46], 3779)            // YpixelsPerM (96 DPI)

	// Pixel data: BMP bottom-to-top, formato B, G, R, A
	idx := 54
	for y := reportBMPH - 1; y >= 0; y-- {
		rowStart := dst.PixOffset(0, y)
		for x := 0; x < reportBMPW; x++ {
			pIdx := rowStart + x*4
			buf[idx] = dst.Pix[pIdx+2]   // B
			buf[idx+1] = dst.Pix[pIdx+1] // G
			buf[idx+2] = dst.Pix[pIdx]   // R
			buf[idx+3] = 0x00           // A
			idx += 4
		}
	}

	return buf, nil
}

// replaceImages reemplaza todas las apariciones de una imagen OLE Picture de tamaño targetLen.
func replaceImages(data []byte, sigHeader []byte, newJPEG []byte) int {
	offsetIntoSig := len(sigHeader) - 2
	targetLen := len(newJPEG)
	count := 0

	for i := 0; i+len(sigHeader) <= len(data); i++ {
		if bytes.Equal(data[i:i+len(sigHeader)], sigHeader) {
			jpegStart := i + offsetIntoSig
			if jpegStart+targetLen <= len(data) {
				copy(data[jpegStart:jpegStart+targetLen], newJPEG)
				count++
				i = jpegStart + targetLen - 1
			}
		}
	}
	return count
}

// hideOverlayLabels neutraliza los controles Label de VB6 superpuestos.
func hideOverlayLabels(data []byte) int {
	patterns := [][]byte{
		[]byte("******* CAPROBA 1981"),
		[]byte("******* CAPROBA"),
		[]byte("CAPROBA 1981"),
		[]byte("CAPROBA"),
	}

	count := 0
	for _, pat := range patterns {
		patLen := len(pat)
		for i := 0; i+patLen <= len(data); i++ {
			if !bytes.Equal(data[i:i+patLen], pat) {
				continue
			}

			propPos := i - 4
			if propPos < 0 || data[propPos] != 0x01 || data[propPos+1] != 0x01 || int(data[propPos+2]) != patLen {
				continue
			}

			distToJPEG := -1
			limit := i + 300
			if limit > len(data)-1 {
				limit = len(data) - 1
			}
			for k := i; k < limit; k++ {
				if data[k] == 0xFF && data[k+1] == 0xD8 {
					distToJPEG = k - i
					break
				}
			}

			if distToJPEG > 0 && distToJPEG < 200 {
				count++
				for j := 0; j < patLen; j++ {
					data[i+j] = 0x20
				}

				coordLimit := i + patLen + 25
				if coordLimit > len(data)-9 {
					coordLimit = len(data) - 9
				}
				for k := i + patLen; k < coordLimit; k++ {
					if data[k] == 0x05 {
						data[k+1] = 0xD0 // Left = -30000
						data[k+2] = 0x8A
						data[k+3] = 0xD0 // Top = -30000
						data[k+4] = 0x8A
						data[k+5] = 0x00 // Width = 0
						data[k+6] = 0x00
						data[k+7] = 0x00 // Height = 0
						data[k+8] = 0x00
						break
					}
				}
			}
		}
	}
	return count
}

// PatchLogos aplica el parche completo de logos y etiquetas sobre el ejecutable en memoria.
func PatchLogos(data []byte, logoImg image.Image) ([]byte, PatchReport, error) {
	if logoImg == nil {
		return nil, PatchReport{}, fmt.Errorf("imagen de logo nula")
	}

	jpegForm, err := GeneratePaddedJPEG(logoImg, logoFormW, logoFormH, logoFormBytes)
	if err != nil {
		return nil, PatchReport{}, fmt.Errorf("generando logo de formularios: %w", err)
	}

	jpegBg, err := GeneratePaddedJPEG(logoImg, logoBgW, logoBgH, logoBgBytes)
	if err != nil {
		return nil, PatchReport{}, fmt.Errorf("generando fondo: %w", err)
	}

	jpegSplash, err := GeneratePaddedJPEG(logoImg, logoSplashW, logoSplashH, logoSplashBytes)
	if err != nil {
		return nil, PatchReport{}, fmt.Errorf("generando splash: %w", err)
	}

	outData := make([]byte, len(data))
	copy(outData, data)

	rep := PatchReport{
		FormLogos:   replaceImages(outData, sigFormLogo, jpegForm),
		Backgrounds: replaceImages(outData, sigBgLogo, jpegBg),
		Splashes:    replaceImages(outData, sigSplash, jpegSplash),
		Labels:      hideOverlayLabels(outData),
	}

	return outData, rep, nil
}

// PatchReportsLogos parchea todas las plantillas .rpt en repDir reemplazando el BMP de 5.678.490 bytes.
func PatchReportsLogos(repDir string, logoImg image.Image, out func(string)) (int, error) {
	if logoImg == nil {
		return 0, fmt.Errorf("imagen de logo nula")
	}
	newBMP, err := GenerateReportBMP(logoImg)
	if err != nil {
		return 0, fmt.Errorf("generando BMP para reportes: %w", err)
	}

	ents, err := os.ReadDir(repDir)
	if err != nil {
		return 0, fmt.Errorf("leyendo directorio de reportes %s: %w", repDir, err)
	}

	targetLen := len(newBMP)
	patchedFiles := 0

	for _, e := range ents {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".rpt") {
			continue
		}
		path := filepath.Join(repDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		count := 0
		for i := 0; i+len(sigReportBMP) <= len(data); i++ {
			if bytes.Equal(data[i:i+len(sigReportBMP)], sigReportBMP) {
				if i+targetLen <= len(data) {
					copy(data[i:i+targetLen], newBMP)
					count++
					i += targetLen - 1
				}
			}
		}

		if count > 0 {
			if err := os.WriteFile(path, data, 0644); err == nil {
				patchedFiles++
			}
		}
	}

	out(fmt.Sprintf("Reportes Crystal: %d plantillas actualizadas con nuevo logo en %s", patchedFiles, repDir))
	return patchedFiles, nil
}

// PatchLogosFile parchea el ejecutable en disco a partir del archivo de imagen.
func PatchLogosFile(exePath, logoPath string, emit func(string)) error {
	logoFile, err := os.Open(logoPath)
	if err != nil {
		return fmt.Errorf("abriendo logo %s: %w", logoPath, err)
	}
	defer logoFile.Close()

	img, _, err := image.Decode(logoFile)
	if err != nil {
		return fmt.Errorf("decodificando logo %s: %w", logoPath, err)
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("leyendo ejecutable %s: %w", exePath, err)
	}

	patched, rep, err := PatchLogos(data, img)
	if err != nil {
		return err
	}

	if rep.Total() == 0 {
		emit(fmt.Sprintf("Sin modificaciones en %s (¿ya estaba parchado?)", filepath.Base(exePath)))
		return nil
	}

	if err := os.WriteFile(exePath, patched, 0644); err != nil {
		return fmt.Errorf("guardando ejecutable parchado %s: %w", exePath, err)
	}

	emit(fmt.Sprintf("Logos parchados en %s: %d formularios, %d fondos, %d splash, %d etiquetas ocultas",
		filepath.Base(exePath), rep.FormLogos, rep.Backgrounds, rep.Splashes, rep.Labels))
	return nil
}
