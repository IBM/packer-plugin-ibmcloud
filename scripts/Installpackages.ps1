iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))

# Globally Auto confirm every action

choco feature enable -n allowGlobalConfirmation

# Install developer packages

choco install python slack
