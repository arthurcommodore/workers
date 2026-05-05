package api

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	ErrInvalidResolution = errors.New("resolução insuficiente para OCR")
	ErrNoTextDetected    = errors.New("imagem não contém texto legível")
	ErrOCRFailed         = errors.New("falha ao extrair texto")
)

var ocrSemaphore = make(chan struct{}, 2)
var pdfSemaphore = make(chan struct{}, 1)

func ProcessUploadedFile(data []byte) (string, error) {
	if !isPDF(data) {
		return "", errors.New("apenas arquivos PDF são aceitos")
	}

	text, err := pdfToText(data)
	if err == nil && looksLikeText(text) {
		return text, nil
	}

	return processPDF(data)
}

func pdfToText(pdfBytes []byte) (string, error) {

	tmpDir, err := os.MkdirTemp("", "pdftext")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	pdfPath := filepath.Join(tmpDir, "doc.pdf")
	if err := os.WriteFile(pdfPath, pdfBytes, 0644); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"pdftotext",
		"-layout",
		pdfPath,
		"-",
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", errors.New("timeout ao extrair texto do pdf")
		}
		return "", err
	}

	return out.String(), nil
}

func processImage(img image.Image) (string, error) {
	if err := validateResolution(img); err != nil {
		return "", err
	}

	text, err := extractTextWithLimit(img)
	if err != nil {
		return "", err
	}

	if !looksLikeText(text) {
		return "", ErrNoTextDetected
	}

	return text, nil
}

func looksLikeText(text string) bool {
	clean := strings.TrimSpace(text)
	if len(clean) < 50 {
		return false
	}

	letters := 0
	for _, r := range clean {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			letters++
		}
	}

	return letters > 30 &&
		float64(letters)/float64(len(clean)) > 0.3
}

func validateResolution(img image.Image) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	if w < 700 || h < 1000 {
		return ErrInvalidResolution
	}
	return nil
}

func extractTextWithLimit(img image.Image) (string, error) {
	ocrSemaphore <- struct{}{}
	defer func() { <-ocrSemaphore }()

	return extractText(img)
}

func extractText(img image.Image) (string, error) {
	var buf bytes.Buffer

	if err := png.Encode(&buf, img); err != nil {
		return "", ErrOCRFailed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"tesseract",
		"stdin",
		"stdout",
		"-l", "por",
		"--psm", "6",
	)

	cmd.Stdin = bytes.NewReader(buf.Bytes())

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return "", ErrOCRFailed
	}

	if err != nil {
		return "", ErrOCRFailed
	}

	return out.String(), nil
}

func isPDF(data []byte) bool {
	limit := 1024
	if len(data) < limit {
		limit = len(data)
	}
	return bytes.Contains(data[:limit], []byte("%PDF"))
}

func processPDF(data []byte) (string, error) {
	tmpDir, err := os.MkdirTemp("", "pdf")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	pdfPath := filepath.Join(tmpDir, "doc.pdf")
	if err := os.WriteFile(pdfPath, data, 0644); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	pdfSemaphore <- struct{}{}
	defer func() { <-pdfSemaphore }()

	cmd := exec.CommandContext(
		ctx,
		"pdftoppm",
		"-png",
		"-r", "300",
		"-gray",
		pdfPath,
		filepath.Join(tmpDir, "page"),
	)

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", errors.New("timeout ao converter PDF")
		}
		return "", err
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", err
	}

	sort.Slice(files, func(i, j int) bool {
		getPageNumber := func(name string) int {
			name = strings.TrimSuffix(name, ".png")
			parts := strings.Split(name, "-")
			if len(parts) < 2 {
				return 0
			}
			n, _ := strconv.Atoi(parts[len(parts)-1])
			return n
		}
		return getPageNumber(files[i].Name()) < getPageNumber(files[j].Name())
	})

	var builder strings.Builder
	validPages := 0
	const maxPages = 50

	for i, f := range files {
		if i >= maxPages {
			break
		}

		if !strings.HasSuffix(f.Name(), ".png") {
			continue
		}

		imgPath := filepath.Join(tmpDir, f.Name())

		fileHandle, err := os.Open(imgPath)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(fileHandle)
		fileHandle.Close()
		if err != nil {
			continue
		}

		text, err := processImage(img)
		if err != nil {
			continue
		}

		builder.WriteString("\n\n--- Página ")
		builder.WriteString(strconv.Itoa(validPages + 1))
		builder.WriteString(" ---\n\n")
		builder.WriteString(text)

		validPages++

		img = nil
	}

	if validPages == 0 {
		return "", ErrNoTextDetected
	}

	return builder.String(), nil
}
