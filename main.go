package main

import (
	"bufio"
	"bytes"
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
	flag.Parse()

	if *parentNode == "" || *refNode == "" {
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
		// Download xml from url
		fmt.Println("Downloading xml from url:", *urlFlag)
		tempDir := os.TempDir()
		tempFilePath := filepath.Join(tempDir, "temp.xml")
		err := downloadFile(*urlFlag, tempFilePath)
		if err != nil {
			fmt.Println("Error downloading xml file:", err)
			return
		}
		defer os.Remove(tempFilePath)
		xmlFilePath = tempFilePath
		fmt.Println("xml file downloaded to:", xmlFilePath)

		// log first bit of file for debugging
		// content, err := os.ReadFile(xmlFilePath)
		// if err != nil {
		// 	fmt.Println("Error reading downloaded XML file:", err)
		// 	return
		// }
		// fmt.Println("Downloaded XML content (first 500 characters):")
		// fmt.Println(string(content[:500]))
	} else {
		// check for required xml in local dir
		xmlFilePath, err = findFileByExtension(dir, ".xml")
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Check for required files
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

		// Write the output XML file
		outputFilePath := filepath.Join(outputDir, "output.xml")
		fmt.Println("Matching entries found. Writing to output.xml...")
		if err := writeToXML(outputFilePath, matchingEntries); err != nil {
			fmt.Println("Error writing to XML file:", err)
		} else {
			fmt.Printf("Captured nodes successfully written to %s\n", outputFilePath)
		}
	}
}

// Locate files in local dir by extension
func findFileByExtension(dir string, extension string) (string, error) {
	fmt.Println("Searching for files in dir:", dir)

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
				if captureDepth != -1 && contains(referenceIDs, text) {
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
func downloadFile(url, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}
