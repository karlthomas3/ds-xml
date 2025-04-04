package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Command-line flags
	parentNode := flag.String("node", "", "Parent node to search for")
	refNode := flag.String("ref", "", "Reference node containing ID")
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

	requiredExtensions := []string{".xml", ".csv"}
	var csvFilePath, xmlFilePath string

	// Check for required files
	for _, ext := range requiredExtensions {
		found := false
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if filepath.Ext(info.Name()) == ext {
				if ext == ".csv" {
					csvFilePath = path
				} else if ext == ".xml" {
					xmlFilePath = path
				}
				found = true
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			fmt.Println("Error walking the directory:", err)
			return
		}
		if !found {
			fmt.Printf("No %s file found.\n", ext)
			return
		}
	}

	// Get IDs from CSV
	var referenceIDs []string
	if csvFilePath != "" {
		fmt.Println("Reading IDs from CSV file:", csvFilePath)
		referenceIDs, err = readCSV(csvFilePath)
		if err != nil {
			fmt.Println("Error reading CSV:", err)
			return
		}
	}

	if xmlFilePath != "" {
		fmt.Println("Parsing XML file:", xmlFilePath)
		matchingEntries, err := parseXML(xmlFilePath, referenceIDs, *parentNode, *refNode)
		if err != nil {
			fmt.Println("Error parsing XML:", err)
			return
		}
		if len(matchingEntries) == 0 {
			fmt.Println("No matching entries found.")
		} else {
			// ensure output folder exists
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
