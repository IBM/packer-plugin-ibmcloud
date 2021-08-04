#ps1_sysnative

# Changes PowerShell execution policies
Set-ExecutionPolicy -ExecutionPolicy Unrestricted -Scope LocalMachine -Force -ErrorAction Ignore
$ErrorActionPreference = "stop"

# Remove any existing Windows Management listeners
Remove-Item -Path WSMan:\Localhost\listener\listener* -Recurse
 
# Create self-signed certificate for encrypted WinRM on port 5986
$Cert = New-SelfSignedCertificate -CertstoreLocation Cert:\LocalMachine\My -DnsName "packer-ibmcloud"
New-Item -Path WSMan:\LocalHost\Listener -Transport HTTPS -Address * -CertificateThumbPrint $Cert.Thumbprint -Force

# Configure WinRM
winrm quickconfig -q
winrm set winrm/config '@{MaxTimeoutms="1800000"}'
winrm set winrm/config/winrs '@{MaxMemoryPerShellMB="1024"}'
winrm set winrm/config/service '@{AllowUnencrypted="false"}'
winrm set winrm/config/client '@{AllowUnencrypted="false"}'
winrm set winrm/config/service/auth '@{Basic="true"}'
winrm set winrm/config/client/auth '@{Basic="true"}'
winrm set winrm/config/service/auth '@{CredSSP="true"}'
winrm set winrm/config/listener?Address=*+Transport=HTTPS "@{Port=`"5986`";Hostname=`"packer-ibmcloud`";CertificateThumbprint=`"$($Cert.Thumbprint)`"}"

# Make sure appropriate firewall port openings exist
netsh advfirewall firewall add rule name="WinRM-SSL (5986)" dir=in action=allow protocol=TCP localport=5986

# Restart WinRM, and set it so that it auto-launches on startup.
Enable-PSRemoting -Force
Stop-Service -Name WinRM
Set-Service -Name WinRM -StartupType Automatic
Start-Service -Name WinRM