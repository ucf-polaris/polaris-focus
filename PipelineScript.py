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
    direc = "TestingPipeline/the_first_go/polaris-test"
    for file in os.listdir(direc):
        if ".go" in file and file != "main_test.go":
            os.remove(direc + "/" + file)

def output_file_menu(output, dic):
    print(output)
    key_index = {}
    
    for index, element in enumerate(dic.keys()):
        print(str(index+1) + ". " + element)
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
    main_file = dirname + '/TestingPipeline/the_first_go/polaris-test/'
    
    #get all go files within directories below this one
    for root, dirs, files in os.walk(directory):
        for file in files:
            if file.endswith(".go"):
                if(main == file):
                    shutil.copyfile(root + "\\" + file, main_file + "main.go")
                else:
                    shutil.copyfile(root + "\\" + file, main_file + file)
                    
def write_to_configs(file):
    dictionary = {"filename": file}
    # Serializing json
    json_object = json.dumps(dictionary, indent=4)
     
    # Writing to sample.json
    with open("TestingPipeline/the_first_go/polaris-test/Helpers/configs.json", "w") as outfile:
        outfile.write(json_object)
        
def main():
    #prepare the input
    prepare_delete()
    dic = scan_directory(".go")

    #get user input
    key_index = output_file_menu("Which file to test?", dic)
    choice = get_choice(len(dic))
    copy_all_go_files(dic[key_index[choice]], key_index[choice])

    json_dict = scan_directory(".json", os.path.dirname(__file__) + "/TestingPipeline/the_first_go/polaris-test/Helpers")
    json_key_index = output_file_menu("Test against what test case?", json_dict)
    json_choice = get_choice(len(json_dict))

    write_to_configs(json_key_index[json_choice])

    #change directories and begin the testing process
    os.chdir("TestingPipeline/the_first_go/polaris-test")

    #reset GOOS so proper testing can occur on windows
    if(platform.system() == "Windows"): subprocess.run(["go", "env", "-w", "GOOS=windows"])

    print("BEGINNING TEST(S)...")
    s = subprocess.getstatusoutput(f'go test -v')[1]
    print(s)

    #reset GOOS back so executable can be compiled correctly
    if(platform.system() == "Windows"): subprocess.run(["go", "env", "-w", "GOOS=linux"])
    
    
if __name__=='__main__':
    main()
    
    
