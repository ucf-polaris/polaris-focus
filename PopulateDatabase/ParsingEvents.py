import requests, json

def main():
    url = "https://knightconnect.campuslabs.com/engage/api/discovery/event/search?take=1&status=Approved&orderByField=endsOn&orderByDirection=ascending&status=Approved"
    r = requests.get(url)

    response = r.json()
    print("total is " + str(response["@odata.count"]))
    location_dict = parseLocations(response["@odata.count"])
    
    
    with open('locationdata.json', 'w') as convert_file: 
     convert_file.write(json.dumps(location_dict))
    
    print("length is " + str(len(location_dict)))

def parseLocations(total):
    url = "https://knightconnect.campuslabs.com/engage/api/discovery/event/search?&take=4000&status=Approved&orderByField=endsOn&orderByDirection=ascending&status=Approved"
    location_dict = {}
    skip = 0    
    while(skip < total):
        r = requests.get(url +"&skip=" + str(skip))
        response = r.json()
        skip += len(response["value"])
        
        for loc in response["value"]:
            if(loc["location"] not in location_dict):
                location_dict[loc["location"]] = 1
            else:
                location_dict[loc["location"]] += 1
            
        print("SKIP TOTAL: " + str(skip))

    return location_dict
    

if __name__=="__main__":
    main()
    
