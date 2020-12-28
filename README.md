# Tripline
## Description

A tool to verify file integrity. Verify if some aspect of a file or directory has changed relative to a recorded base situation.

## Usage
### Add file/directory information

```
tripline add (FILE|DIR)+
    
Example
$ tripline add ~/.ssh
```
    
Add options
* **--fileset NAME**. 
   * The fileset to add the information to. 
   * The fileset will be created automatically if it does not exist.
   * Default value: "default".
* **--recursive BOOL**. 
   * Indicate if directories should be added recursively. 
   * Default: true.
* **--skip BOOL**, **--overwrite BOOL**. 
   * You have to indicate explicitly what should happen if file information is already in the database. There is no default value.
* **--dirchecks CHECKLIST**, **--filechecks CHECKLIST**. 
   * The checks you want to perform on the added files and directories.
   * File default: size,modtime,ownership,permissions,sha256
   * Dir default: child,modtime,ownership,permissions


```bash
tripline delete (FILE|DIR)+

Example
tripline delete ~/./ssh/id.pub
```

Delete options
* **--fileset NAME**. 
   * The fileset from which to delete the files and directories.
   * The command always works recursively.

### Verify integrity

    tripline verify (FILE|DIR)*
    
    Example
    $ tripline verify

    
Verify options
* **--fileset NAME**. 
   * The fileset to use for the verification. 
   * Default: "default".    
   * Explicit file and directory arguments are optional. If no files or directories are provided the complete fileset will be verified.

### Fileset maintenance

List the contents of a fileset
* List options
    * **--fileset NAME**.
    

    tripline list

  
Delete a fileset
* Delete options
    * **--fileset NAME**.


    tripline deleteset


Copy a fileset. Can be handy before making modifications.
* Copyset options
    * **--fileset NAME**.


    tripline copyset TO


List the available datasets
* Listsets options
    * **--fileset NAME**.


    tripline listsets

## Signatures

Protect against database tampering with signatures. It is a manual process, the signatures are not automatically 
verified by the other operations. 

    tripline sign
    tripline verifysig

Options
* --fileset NAME
* --overwrite BOOL


## Improvements

* Add multithreading to parallelize the testing.
* Add options to configure the processing of symbolic links. Sometimes you might want to handle the symlinks as files.
* Port to other platforms mswin, osx, ...
* Add unit tests