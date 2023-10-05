import os, zipfile, time, subprocess

def partial_match(file):
    just_name = file.split(".")[0]
    partial_match = ["helper_", "_test"]
    
    for i in partial_match:
        if i in just_name.lower():
            return True

def full_match(file):
    just_name = file.split(".")[0]
    full_match = ["template", "unique", "helpers", "main", "schema", "configs"]
    
    for i in full_match:
        if i == just_name.lower():
            return True
def scan_directory(extension, current_location=""):
    if(current_location == ""): current_location = os.path.dirname(__file__)

    paths = {}

    #get all go files within directories below this one
    for root, dirs, files in os.walk(current_location):
        for file in files:
            if file.endswith(extension) and not full_match(file) and not partial_match(file):
                paths[file] = root
                
    return paths

def main():
    paths = scan_directory(".go")
    for i in paths.keys():
        os.chdir(paths[i])
        print("BUILDING " + i + "...")
        subprocess.Popen(["go","build",i], shell=True)

if __name__ == '__main__':
    main()
