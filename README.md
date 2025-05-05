# XML Parser Tool

## Overview

This tool is designed to parse an XML file and extract specific parent nodes
based on matching child node values provided in a CSV file. The extracted nodes
are written to a new XML file, wrapped in a root element, and saved in an
`output` directory.

---

## Features

- Extracts parent nodes (`-node`) containing child nodes (`-ref`) with matching
  values from a CSV file.
- Outputs the extracted nodes as a well-formed XML file with a root element.
- Use the (`-url`) flag to download (and if needed extract from .zip, .gz, or
  .tar.gz) to a temp directory for parsing. (Downloaded files are automatically
  cleaned after use)
- Automatically creates an `output` directory to store the results.

---

## Requirements

- Go 1.16 or later
- Input files:
  - An XML file containing the data to parse.
  - A CSV file containing the reference IDs.

---

## Installation

In your terminal type:

`git clone https://github.com/karlthomas3/ds-xml.git`

`cd ds-xml`

`go build`

---

## Usage

### Warning

- You must place the `.xml` you wish to parse in the same location as ds-xml and
  ensure it's the only `.xml` in that location (if parsing local xml)
- You must place a `csv` containing the IDs for the entries you wish to extract
  in the same location as ds-xml and ensure it't the only `.csv`

### Command-Line Flags

- `-node`: The name of the parent node to search for in the XML file.
- `-ref`: The name of the child node containing the reference ID.
- (If no `-ref` is provided, then ALL nodes will match.)
- `-head`: scans the first N characters and prints them to the console. Useful
  for discovering unknown tag names for `-node` and `-ref`
- `-url`: The url to download the xml from.

### Steps to Run

1. Place the XML and CSV files in the same directory as the executable.
2. Run the tool using the following command:
   ```bash
   ./ds-xml -node <ParentNodeName> -ref <ChildNodeName>
   ```

- Replace `<ParentNodeName>` with the name of the parent node (e.g. `job`) and
  `<ChildNodeName>` with the name of the child node (e.g., `job_reference`).

#### Example:

If you have:

- An XML file named sample.xml with <job> as the parent node and <job_reference>
  as the child node.
- A CSV file named ref.csv containing reference IDs. Run the tool as:
  `./ds-xml -node job -ref job_reference`

#### Output

- The tool creates an output directory in the current working directory.
- The extracted nodes are written to output/output.xml.
  ##### Example Output
  If the input XML contains:
  ```
  <job>
  <location>Las Vegas, NV</location> <job_reference>12345</job_reference>
  </job>
  <job>
  <location>New York, NY</location> <job_reference>67890</job_reference>
  </job>
  ```
  And the CSV contains:
  ```
  12345
  ```
  The `output/output.xml` will contain:

  ```
  <?xml version="1.0" encoding="UTF-8"?>
  <root>
  <job>
    <location>Las Vegas, NV</location>
    <job_reference>12345</job_reference>
  </job>
  </root>
  ```

---

### Error Handling

- If the required files(`.xml` or `.csv`) are not found, the tool will display
  an error and exit.
- If no matching nodes are found, the tool will display:
  ```
  No matching entries found.
  ```
- If the `output` directory cannot be created, the tool will display an error.

---

### Development notes

- The tool uses Go's `encoding/xml` pacage for the XML parsing and writing.
- The filepath.Walk function is used to locate `.xml` and `.csv` files in the
  current directory
- The `os.MkdirAll` function ensures the `output` directory is created if it
  doesn't already exist.

---

### Future Enhancements

- Add support for specifying input file paths via command-line flags.
- Add support for specifying URL via command-line flag so `xml` can be
  downloaded and parsed automatically.
- Allow customization of the output directory and file name.
- Allow for unzipping of compressed XMLs.
- Add logging for better debugging and traceability.

---

### License

This tool is open-source and free to use. Modify and distribute as needed.
Contributions are welcome!
