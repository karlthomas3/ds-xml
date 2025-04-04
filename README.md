# XML Parser Tool

---

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
- Automatically creates an `output` directory to store the results.

---

## Requirements

- Go 1.16 or later
- Input files:
  - An XML file containing the data to parse.
  - A CSV file containing the reference IDs.

---

## Installation

- In your terminal type: `git clone https://github.com/karlthomas3/ds-xml`

---

## Usage

### Command-Line Flags

- `-node`: The name of the parent node to search for in the XML file.
- `-ref`: The name of the child node containing the reference ID.

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
