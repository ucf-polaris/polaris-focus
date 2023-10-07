import os
import subprocess

# define all the root directory names
roots = ['LocationEndpoints', 'EventEndpoints', 'UserEndpoints', 'JWTFiles', 'Scan']
# map each root directory to its list of endpoints
functions = {
    'LocationEndpoints': ['BuildingAdd', 'BuildingGet', 'BuildingDelete', 'BuildingUpdate'],
    'EventEndpoints': ['EventGet', 'EventsAdd', 'EventsDelete', 'EventsUpdate'],
    'UserEndpoints': ['RegistrationCode', 'Register', 'Login', 'UserGet', 'UserUpdate', 'UserDelete'],
    'JWTFiles': ['TokenVerify'],
    'Scan': ['Scan']
}

# go through each of the roots
for root in roots:
    # go through each function contained in this root
    for func in functions[root]:
        # run proper go build command for this endpoint
        print(f'Building {root}/{func}')
        cmd = f'GOOS=linux GOARCH=amd64 go build -o {root}/{func}/{func} {root}/{func}/{func}.go'
        subprocess.run(cmd, shell=True)

# define file names for sam package and deploy so it can be easily modified
template_file = 'template.yaml'
output_template = 'packaged.yaml'
s3_bucket = 'aws-sam-cli-managed-default-samclisourcebucket-ri5k5ky9x3uv'
sam_config = 'samconfig.toml'
# package the template so it can be deployed
cmd = f'sam package --template-file {template_file} --output-template-file {output_template} --s3-bucket {s3_bucket}'
subprocess.run(cmd, shell=True)
cmd = f'sam deploy --config-file {sam_config}'
subprocess.run(cmd, shell=True)