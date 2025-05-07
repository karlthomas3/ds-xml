package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Command-line flags
	parentNode := flag.String("node", "", "Parent node to search for")
	refNode := flag.String("ref", "", "Reference node containing ID")
	urlFlag := flag.String("url", "", "URL to download xml from")
	scanFlag := flag.Int("head", 0, "Scan and print the first N characters of the xml")
	chunkSize := flag.Int("chunk", 0, "Number of entries per output xml file (default: all in one file)")
	flag.Parse()

	if *parentNode == "" {
		fmt.Println("Usage: ds-xml -node <parentNode> -ref <refNode>")
		return
	}

	// Get local dir
	execPath, err := os.Executable()
	if err != nil {
		fmt.Println("Error getting executable path:", err)
		return
	}
	dir := filepath.Dir(execPath)

	var xmlFilePath string

	if *urlFlag != "" {
		// Download from url
		fmt.Println("Downloading file from url:", *urlFlag)
		tempDir := os.TempDir()

		// extract filename from url
		fileName := filepath.Base(*urlFlag)
		tempFilePath := filepath.Join(tempDir, fileName)

		// download and extract file
		xmlFilePath, err = downloadFile(*urlFlag, tempFilePath)
		if err != nil {
			fmt.Println("Error downloading xml file:", err)
			return
		}
		defer os.Remove(xmlFilePath)
		fmt.Println("xml file downloaded to:", xmlFilePath)

		// check if file exists
		if _, err := os.Stat(xmlFilePath); os.IsNotExist(err) {
			fmt.Println("Error: Extracted XML file does not exist:", xmlFilePath)
			return
		}

	} else {
		// check for required xml in local dir
		xmlFilePath, err = findFileByExtension(dir, ".xml")
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	if *scanFlag > 0 {
		content, err := os.ReadFile(xmlFilePath)
		if err != nil {
			fmt.Println("Error reading XML file:", err)
			return
		}
		fmt.Printf("Scanned XML content (first %d characters):\n", *scanFlag)
		fmt.Println(string(content[:min(*scanFlag, len(content))]))
		return
	}

	// Check for csv
	csvFilePath, err := findFileByExtension(dir, ".csv")
	if err != nil {
		fmt.Println(err)
		return
	}

	// Get IDs from CSV
	fmt.Println("Reading IDs from CSV file:", csvFilePath)
	referenceIDs, err := readCSV(csvFilePath)
	if err != nil {
		fmt.Println("Error reading CSV:", err)
		return
	}

	// Parse XML
	fmt.Println("Parsing XML file:", xmlFilePath)
	matchingEntries, err := parseXML(xmlFilePath, referenceIDs, *parentNode, *refNode)
	if err != nil {
		fmt.Println("Error parsing XML:", err)
		return
	}

	if len(matchingEntries) == 0 {
		fmt.Println("No matching entries found.")
	} else {
		// Ensure output folder exists
		outputDir := "output"
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			fmt.Println("Error creating output directory:", err)
			return
		}

		// handle chunking
		totalEntries := len(matchingEntries)
		chunk := *chunkSize
		if chunk <= 0 || chunk > totalEntries {
			chunk = totalEntries
		}

		for i := 0; i < totalEntries; i += chunk {
			end := i + chunk
			if end > totalEntries {
				end = totalEntries
			}

			// generate output file name for chunk
			refPart := *refNode
			if refPart == "" {
				refPart = "all"
			}
			outputFileName := fmt.Sprintf("%s_%s_part-%d.xml", *parentNode, refPart, i/chunk+1)

			// Write the output XML file
			outputFilePath := filepath.Join(outputDir, outputFileName)
			fmt.Printf("Writing chunk %d to %s ... \n", i/chunk+1, outputFilePath)
			if err := writeToXML(outputFilePath, matchingEntries[i:end]); err != nil {
				fmt.Printf("Error writing chunk %d to XML file: %v\n", i/chunk+1, err)
			} else {
				fmt.Printf("Captured nodes successfully written to %s\n", outputFilePath)
			}
		}
	}
}

// Locate files in local dir by extension
func findFileByExtension(dir string, extension string) (string, error) {
	fmt.Printf("Searching for %s in dir: %s\n", extension, dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("Error reading directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == extension {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("No %s file found in directory: %s", extension, dir)
}

// Reads CSV and returns slice of IDs
func readCSV(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ids []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		split := strings.Split(line, ",")
		for _, id := range split {
			id = strings.TrimSpace(id)
			if id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids, scanner.Err()
}

func parseXML(filePath string, referenceIDs []string, parentNode, refNode string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var results []string
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var currentDepth int
	var buffer bytes.Buffer
	var encoder *xml.Encoder
	var captureDepth = -1
	var insideParent bool
	var matchFound bool

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			currentDepth++
			if t.Name.Local == parentNode {
				// Start capturing the parent node
				insideParent = true
				captureDepth = currentDepth
				buffer.Reset()
				encoder = xml.NewEncoder(&buffer)
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
				// if no refNode provided, consider all parent nodes a match
				if refNode == "" {
					matchFound = true
				}
			} else if insideParent {
				// Capture child nodes of the parent
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if insideParent {
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
				if t.Name.Local == parentNode && currentDepth == captureDepth {
					// End of the parent node
					if matchFound {
						if err := encoder.Flush(); err != nil {
							return nil, err
						}
						results = append(results, buffer.String())
					}
					// Reset state for the next parent node
					buffer.Reset()
					insideParent = false
					captureDepth = -1
					matchFound = false
				}
			}
			currentDepth--
		case xml.CharData:
			if insideParent {
				text := strings.TrimSpace(string(t))
				if refNode != "" && captureDepth != -1 && contains(referenceIDs, text) {
					matchFound = true
				}
				if err := encoder.EncodeToken(t); err != nil {
					return nil, err
				}
			}
		}
	}

	return results, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Writes to an XML file
func writeToXML(filePath string, capturedNodes []string) error {
	// Create or overwrite the XML
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Error creating XML file: %v", err)
	}
	defer file.Close()

	// Write XML declaration
	_, err = file.WriteString(xml.Header)
	if err != nil {
		return fmt.Errorf("Error writing XML header: %v", err)
	}

	// Write opening root element
	_, err = file.WriteString("<root>\n")
	if err != nil {
		return fmt.Errorf("Error writing root element: %v", err)
	}

	// Write each captured node to file
	for _, node := range capturedNodes {
		_, err := file.WriteString(node + "\n")
		if err != nil {
			return fmt.Errorf("Error writing to XML file: %v", err)
		}
	}

	// Write closing root element
	_, err = file.WriteString("</root>\n")
	if err != nil {
		return fmt.Errorf("Error writing closing root element: %v", err)
	}
	return nil
}

// Downloads a file from a URL and saves it to the specified path
// handles .zip, .gz, and .tar.gz.
func downloadFile(url, filePath string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	// Handle compressed files based on their extensions
	switch {
	case strings.HasSuffix(filePath, ".zip"):
		fmt.Println("File is a ZIP archive. Extracting...")
		extractedFiles, err := unzip(filePath, filepath.Dir(filePath))
		if err != nil {
			return "", fmt.Errorf("failed to extract ZIP file: %v", err)
		}
		err = os.Remove(filePath) // Delete the ZIP file after extraction
		if err != nil {
			return "", fmt.Errorf("failed to delete ZIP file: %v", err)
		}
		// Return the first extracted file (assuming it's the XML file)
		return extractedFiles[0], nil

	case strings.HasSuffix(filePath, ".gz") && !strings.HasSuffix(filePath, ".tar.gz"):
		fmt.Println("File is a GZIP archive. Extracting...")
		extractedFilePath := strings.TrimSuffix(filePath, ".gz")
		extractedFile, err := ungzip(filePath, extractedFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to extract GZIP file: %v", err)
		}
		err = os.Remove(filePath) // Delete the GZIP file after extraction
		if err != nil {
			return "", fmt.Errorf("failed to delete GZIP file: %v", err)
		}
		return extractedFile, nil

	case strings.HasSuffix(filePath, ".tar.gz") || strings.HasSuffix(filePath, ".tgz"):
		fmt.Println("File is a TAR.GZ archive. Extracting...")
		extractedFiles, err := untarGz(filePath, filepath.Dir(filePath))
		if err != nil {
			return "", fmt.Errorf("failed to extract TAR.GZ file: %v", err)
		}
		err = os.Remove(filePath) // Delete the TAR.GZ file after extraction
		if err != nil {
			return "", fmt.Errorf("failed to delete TAR.GZ file: %v", err)
		}
		// Return the first extracted file (assuming it's the XML file)
		return extractedFiles[0], nil
	}

	// If the file is not compressed, return the original file path
	return filePath, nil
}

// Unzips compressed files
func unzip(src, dest string) ([]string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var extractedFiles []string

	for _, f := range r.File {
		fPath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fPath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("illegal file path: %s", fPath)
		}

		if f.FileInfo().IsDir() {
			// Create directories
			if err := os.MkdirAll(fPath, os.ModePerm); err != nil {
				return nil, err
			}
			continue
		}

		// Create files
		if err := os.MkdirAll(filepath.Dir(fPath), os.ModePerm); err != nil {
			return nil, err
		}

		outFile, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return nil, err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return nil, err
		}

		extractedFiles = append(extractedFiles, fPath)
	}

	return extractedFiles, nil
}

func ungzip(src, dest string) (string, error) {
	fmt.Printf("Extracting .gzip file: %s to %s\n", src, dest)

	file, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("failed to open .gzip file: %v", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gz.Close()

	outFile, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("failed to create extracted file: %v", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, gz)
	if err != nil {
		return "", fmt.Errorf("failed to extract .gzip file: %v", err)
	}

	fmt.Printf(".gzip file extracted to %s\n", dest)
	return dest, nil
}

func untarGz(src, dest string) ([]string, error) {
	file, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tarReader := tar.NewReader(gz)
	var extractedFiles []string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return nil, err
		}

		fPath := filepath.Join(dest, header.Name)
		if !strings.HasPrefix(fPath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("illegal file path: %s", fPath)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directories
			if err := os.MkdirAll(fPath, os.ModePerm); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			// Create files
			if err := os.MkdirAll(filepath.Dir(fPath), os.ModePerm); err != nil {
				return nil, err
			}
			outFile, err := os.Create(fPath)
			if err != nil {
				return nil, err
			}
			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return nil, err
			}
			extractedFiles = append(extractedFiles, fPath)
		}
	}

	return extractedFiles, nil
}
