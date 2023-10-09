import json, numpy, re
import pandas as pd

def main():
    #get json data
    file = open("./locationdata.json")
    data = json.load(file)
    file.close()

    truth = pd.read_csv("./Polaris-Locations.csv").to_dict('records')
    
    unfound_res = open("unfound.txt", "w")
    unfound = set()
    
    found_res = open("found.txt", "w")
    
    print("amount of data is " + str(len(data.keys())))

    for location in data.keys():
        ret = None
        # 1. full Name
        # 2. partial Name
        # 3. partial Abbreviation
        # 4. partial Alias
        ret = checkFullName(location, truth, found_res)
        if(ret == None): ret = checkPartialAbbre(location, truth, found_res)
        if(ret == None): ret = checkPartialAlias(location, truth, found_res)
        if(ret == None): ret = checkPartialName(location, truth, found_res)
        
        if(ret == None): 
            unfound.add(location)
    
    unfound_res.write('\n'.join(unfound))
    unfound_res.close()
    found_res.close()

def findWholeWord(w):
    return re.compile(r'\b({0})\b'.format(w), flags=re.IGNORECASE).search

def ridNumbers(s):
    return re.sub(r'[0-9]+', ' ', s)

def stripSpecial(s):
	return re.sub(r'[^A-Za-z0-9 ]+', ' ', s)

def checkFullName(location, truth, found):
    for record in truth:
        if(record["Name"] == location):
            found.write("1. FULL Found " + location + " ---  " + record["Name"] + "\n")
            return record["Name"]
    return None

def checkPartialName(location, truth, found):
    for record in truth:
        
        use = record["Name"]
        if(type(use) == str): 
            use = stripSpecial(use)
            
        if(findWholeWord(use)(stripSpecial(location)) != None):
            found.write("2. PARTIAL Found " + location + " --- " + record["Name"] + "\n")
            return record["Name"]
    return None

def checkPartialAbbre(location, truth, found):
    for record in truth:
        if(type(record["Abbreviation"]) != str):
            continue
        
        abbres = record["Abbreviation"].split(",")
        for abb in abbres:
            #first try with whole location (strip special characters)
            if(findWholeWord(abb)(stripSpecial(location)) != None):
                found.write("3. ABBRE1 Found " + location + " --- " + abb + " (" + record["Name"] +")\n")
                return record["Name"]
            
            #try with location without numbers
            if(findWholeWord(abb)(stripSpecial(ridNumbers(location))) != None):
                found.write("3. ABBRE2 Found " + location + " --- " + abb + " (" + record["Name"] +")\n")
                return record["Name"]
            
            #check if first three characters match
            """if(abb.lower() == location[:4].lower()):
                found.write("3. ABBRE3 Found " + location + " --- " + abb + " (" + record["Name"] +")\n")
                return record["Name"]"""
    return None

def checkPartialAlias(location, truth, found):
    for record in truth:
        if(type(record["Alias"]) != str):
            continue
        aliases = record["Alias"].split(",")
        for alias in aliases:
            if(findWholeWord(alias)(stripSpecial(location)) != None):
                found.write("4. ALIAS Found " + location + " --- " + alias + " (" + record["Name"] +")\n")
                return record["Name"]
    return None
        
    
if __name__=="__main__":
    main()

    
