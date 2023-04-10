Write-Host "PACKER_BUILD_NAME is an env var Packer automatically sets for you."
Write-Host "...or you can set it in your builder variables."
Write-Host "The default for this builder is:" $Env:PACKER_BUILD_NAME

Write-Host "The PowerShell provisioner will automatically escape characters"
Write-Host "considered special to PowerShell when it encounters them in"
Write-Host "your environment variables or in the PowerShell elevated"
Write-Host "username/password fields."
Write-Host "For example, VAR1 from our config is:" $Env:VAR1
Write-Host "Likewise, VAR2 is:" $Env:VAR2
Write-Host "VAR3 is:" $Env:VAR3
Write-Host "Finally, VAR4 is:" $Env:VAR4
Write-Host "None of the special characters needed escaping in the template"
mkdir "C:\\tempTest\\"