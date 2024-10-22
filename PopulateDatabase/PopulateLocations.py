import pandas as pd
import numpy, json, requests

def get_json():
    #get data in
    df = pd.read_csv("./Polaris-Locations.csv")
    
    #drop non-schema columns
    pd.set_option("display.precision", 15)
    
    #get correct long and lat
    df[['BuildingLat', 'BuildingLong']] = df["Latitude, Longitude"].str.split(",", n=1,expand=True)
    #df = df.drop("Latitude, Longitude", axis=1)
    
    #get to float
    df['BuildingLat'] = df['BuildingLat'].astype(float)
    df['BuildingLong'] = df['BuildingLong'].astype(float)
    df = df.rename(columns={"Name": "BuildingName", "Informative Text": "BuildingDesc",
                            "Image":"BuildingImage","Altitude": "BuildingAltitude","Abbreviation":"BuildingAbbreviation",
                            "Alias":"BuildingAllias","Address": "BuildingAddress", "Location Type":"BuildingLocationType"})
    #strings to lists
    df["BuildingAllias"] = df["BuildingAllias"].str.split(",")
    df["BuildingAbbreviation"] = df["BuildingAbbreviation"].str.split(",")
    
    #convert to json
    
    df_json = df.to_json(double_precision=15, orient='records')
    ret = json.loads(df_json)
    
    #clean up nulls
    remove_null(ret)
    
    return ret

def remove_null(js_list):
    #remove null entries in list of dictionary
    for i in range(len(js_list)):
        
        copy = js_list[i].copy()
        
        for key, value in js_list[i].items():
            if value == None:  
                del copy[key]
        js_list[i] = copy
        
def populate_database(js_list):
    api_url = input("provide api url: ")
    
    headers = {"Content-Type":"application/json",
               "authorizationToken":"{\"token\":\"eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3Mjg1NDUwOTF9.RPs4A5MjKsXqxIpR4ZL5xKsyqzcI8jqWuCXXKivFMWoghpD3KYdas-FXwv8MfE0kFmc1x3o5fWCEaU6xZwe_zg\"}"}
    
    for js in js_list:
        print(js)
        js["DoOverride"] = True;
        response = requests.post(api_url, data=json.dumps(js), headers=headers)
            
        if("BuildingName" in js):
            print(js["BuildingName"] + ": ", response.text+"\n")
        else:
            print(response.text+"\n")

def main():
    js = get_json()
    populate_database(js)
    
if __name__ == '__main__':
    main()
