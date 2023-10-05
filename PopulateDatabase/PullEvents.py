import requests
r = requests.get('https://knightconnect.campuslabs.com/engage/api/discovery/event/search?endsAfter=2023-10-3&orderByField=endsOn&orderByDirection=ascending&status=Approved')
response = r.json()

print(response['value'])
