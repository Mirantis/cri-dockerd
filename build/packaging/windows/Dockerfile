# The number of build steps below are explicitly minimised to improve performance.
ARG GO_IMAGE
FROM ${GO_IMAGE}

# Install chocolatey (s/o @stefanscherer)
ENV chocolateyUseWindowsCompression false

RUN iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1')); \
    choco feature disable --name showDownloadProgress

RUN choco install -y make

# Use PowerShell as the default shell
SHELL ["powershell", "-Command", "$ErrorActionPreference = 'Stop'; $ProgressPreference = 'SilentlyContinue';"]

# Environment variable notes:
#  - FROM_DOCKERFILE is used for detection of building within a container.
ENV FROM_DOCKERFILE=1

RUN setx /M PATH $($Env:PATH+';C:\go\bin');

RUN New-Item -ItemType Directory -Path C:\go\src\github.com\docker\docker | Out-Null;
RUN C:\git\cmd\git config --global core.autocrlf true;

# Make PowerShell the default entrypoint
ENTRYPOINT ["powershell.exe"]
