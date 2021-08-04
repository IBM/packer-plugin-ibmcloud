# Install Chocolatey
# https://chocolatey.org/docs/installation#install-with-powershellexe
# Need the SecurityProtocol bit because the Chocolatey.org site only responds to TLS1.2 now
# https://chocolatey.org/blog/remove-support-for-old-tls-versions
Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))
# Globally Auto confirm every action
choco feature enable -n allowGlobalConfirmation

# Install Python and slack
choco install python slack