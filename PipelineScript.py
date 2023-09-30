import shutil, os, subprocess, platform, json

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

def prepare_delete():
    direc = "polaris-test/"
    for file in os.listdir(direc):
        if ".go" in file and file != "main_test.go":
            os.remove(direc + "/" + file)

def output_file_menu(output, dic, GUI=True):
    if(GUI): print(output)
    key_index = {}
    
    for index, element in enumerate(dic.keys()):
        if(GUI): print(str(index+1) + ". " + element)
        key_index[str(index+1)] = element

    return key_index

def get_choice(max_items):
    choice = -1
    while(True):
        choice = input()

        if(not choice.isnumeric() or int(choice) > max_items or int(choice) < 1):
            print("INVALID INPUT\n")
        else:
            break
        
    return choice

def copy_all_go_files(directory, main):
    dirname = os.path.dirname(__file__)
    main_file = os.path.join(dirname, 'polaris-test') + '/'
    sep = "\\" if platform.system() == "Windows" else "/"
    
    #get all go files within directories below this one
    for root, dirs, files in os.walk(directory):
        for file in files:
            if file.endswith(".go"):
                if(main == file):
                    shutil.copyfile(root + sep + file, main_file + "main.go")
                else:
                    shutil.copyfile(root + sep + file, main_file + file)
                    
def write_to_configs(file):
    dictionary = {"filename": file}
    # Serializing json
    json_object = json.dumps(dictionary, indent=4)
     
    # Writing to sample.json
    with open("polaris-test/test-cases/configs.json", "w") as outfile:
        outfile.write(json_object)
        
def main():
    first_run = True
    choice, json_choice = 0, 0
    while(True):
        current = os.path.dirname(__file__)
        print(current)
        
        #prepare the input
        prepare_delete()
        dic = scan_directory(".go")

        #get user input
        key_index = output_file_menu("Which file to test?", dic, first_run)
        if(first_run == True):
            choice = get_choice(len(dic))
        copy_all_go_files(dic[key_index[choice]], key_index[choice])

        json_dict = scan_directory(".json", os.path.dirname(__file__) + "/polaris-test/test-cases")
        json_key_index = output_file_menu("Test against what test case?", json_dict, first_run)
        if(first_run == True):
            json_choice = get_choice(len(json_dict))

        write_to_configs(json_key_index[json_choice])

        os.chdir("polaris-test")
        
        run_test()
        first_run = False

        os.chdir(current)
        
        try_again = input("type 'a' to test again OR press key to close...")
        if(try_again != "a"):
            break

def run_test():
    #change directories and begin the testing process
    

    #reset GOOS so proper testing can occur on windows
    if(platform.system() == "Windows"): subprocess.run(["go", "env", "-w", "GOOS=windows"])

    print("BEGINNING TEST(S)...")
    s = subprocess.getstatusoutput(f'go test -v')[1]
    print(s)

    #reset GOOS back so executable can be compiled correctly
    if(platform.system() == "Windows"): subprocess.run(["go", "env", "-w", "GOOS=linux"])
    
if __name__=='__main__':
    main()
    
    
