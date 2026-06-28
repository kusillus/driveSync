## ADDED Requirements

### Requirement: Traverse Subdirectories Recursively
The system SHALL scan all directories under the mapped local directory recursively, excluding hidden files and directories whose names start with a dot, to discover nested files.

#### Scenario: Recursive scan finds files in nested subdirectories
- **WHEN** the local scanner runs on a local path containing a nested directory `subfolder` with file `file.txt`
- **THEN** the system SHALL detect `file.txt` with its relative path from the mapped root directory.

### Requirement: Preserve Directory Hierarchy on Google Drive Upload
The system SHALL create any missing parent subfolders in the file's path recursively on Google Drive under the mapped Drive folder ID before uploading the file, and upload the file with its parent set to the final subfolder ID.

#### Scenario: Upload nested file to Drive
- **WHEN** a file with a relative path `folder1/folder2/file.txt` is pending upload
- **THEN** the system SHALL resolve or create `folder1` inside the root Drive folder, resolve or create `folder2` inside `folder1`, and upload `file.txt` inside `folder2`.

### Requirement: Preserve Directory Hierarchy on Local Download
The system SHALL recreate any missing parent directories locally before downloading a file that is nested under subfolders on Google Drive.

#### Scenario: Download nested file from Drive
- **WHEN** a remote file located in `folder1/folder2/file.txt` is pending download
- **THEN** the system SHALL create the local directory structure `folder1/folder2` if it does not exist, and save the downloaded file to `folder1/folder2/file.txt`.
