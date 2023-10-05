import pandas as pd
import numpy, json, requests

def get_json():
    #get data in
    df = pd.read_csv("./Polaris-Locations.csv")
    
    #drop non-schema columns
    df = df.drop("Anchor Type", axis=1)
    df = df.drop("Location Type", axis=1)
    df = df.drop("Altitude", axis=1)
    pd.set_option("display.precision", 15)
    
    #get correct long and lat
    df[['BuildingLat', 'BuildingLong']] = df["Latitude, Longitude"].str.split(",", n=1,expand=True)
    #df = df.drop("Latitude, Longitude", axis=1)
    
    #get to float
    df['BuildingLat'] = df['BuildingLat'].astype(float)
    df['BuildingLong'] = df['BuildingLong'].astype(float)
    df = df.rename(columns={"Location Name": "BuildingName", "Informative Text": "BuildingDesc"})
    
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
        
def add_override(js_list):
    for i in range(len(js_list)):
        js_list[i]["DoOverride"] = True
        
def populate_database(js_list):
    api_url = input("provide api url: ")
    
    headers = {"Content-Type":"application/json", "authorizationToken":""}
    
    for js in js_list:
        response = requests.post(api_url, data=json.dumps(js), headers=headers)
        print(js["BuildingName"] + ": ", response.text)

def main():
    js = get_json()
    add_override(js)
    populate_database(js)
    
if __name__ == '__main__':
    main()