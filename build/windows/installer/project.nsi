Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"
!include "StrFunc.nsh"
!include "LogicLib.nsh"
!include "nsDialogs.nsh"
${StrRep}

Var UninstallRemoveData
Var UninstallRemoveDataCheckbox

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_COMPONENTS # Optional current-user startup registration.
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

UninstPage custom un.RemoveDataPage un.RemoveDataPageLeave
!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## Production release builds pass -DFLUXDM_SIGN_INSTALLER and provide the three environment values below.
!ifdef FLUXDM_SIGN_INSTALLER
!ifdef FLUXDM_TEST_UNTIMESTAMPED
!uninstfinalize '"$%FLUXDM_SIGNTOOL%" sign /sha1 "$%FLUXDM_CERT_THUMBPRINT%" /fd SHA256 "%1"'
!finalize '"$%FLUXDM_SIGNTOOL%" sign /sha1 "$%FLUXDM_CERT_THUMBPRINT%" /fd SHA256 "%1"'
!else
!uninstfinalize '"$%FLUXDM_SIGNTOOL%" sign /sha1 "$%FLUXDM_CERT_THUMBPRINT%" /fd SHA256 /tr "$%FLUXDM_TIMESTAMP_URL%" /td SHA256 "%1"'
!finalize '"$%FLUXDM_SIGNTOOL%" sign /sha1 "$%FLUXDM_CERT_THUMBPRINT%" /fd SHA256 /tr "$%FLUXDM_TIMESTAMP_URL%" /td SHA256 "%1"'
!endif
!endif

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Function un.RemoveDataPage
    !insertmacro MUI_HEADER_TEXT "FluxDM user data" "Choose whether to remove local application data."
    nsDialogs::Create 1018
    Pop $0
    ${If} $0 == error
        Abort
    ${EndIf}
    ${NSD_CreateCheckbox} 0 0 100% 24u "Remove FluxDM settings, history, credentials, recovery backups, and logs"
    Pop $UninstallRemoveDataCheckbox
    ${NSD_SetState} $UninstallRemoveDataCheckbox ${BST_UNCHECKED}
    ${NSD_CreateLabel} 0 34u 100% 42u "Downloaded files and unrecognized files are never removed. Leave this unchecked to preserve FluxDM data for reinstalling."
    Pop $0
    nsDialogs::Show
FunctionEnd

Function un.RemoveDataPageLeave
    ${NSD_GetState} $UninstallRemoveDataCheckbox $UninstallRemoveData
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    File /oname=FluxDM.NativeHost.exe "..\..\bin\FluxDM.NativeHost.exe"
    SetOutPath "$INSTDIR\browser-extension"
    File /r "..\..\..\browser-extension\*.*"
    SetOutPath $INSTDIR

    ${StrRep} $0 "$INSTDIR\FluxDM.NativeHost.exe" "\" "/"
    FileOpen $1 "$INSTDIR\com.fluxdm.browser.json" w
    FileWrite $1 '{$\r$\n  "name": "com.fluxdm.browser",$\r$\n  "description": "FluxDM native messaging host",$\r$\n  "path": "$0",$\r$\n  "type": "stdio",$\r$\n  "allowed_origins": ["chrome-extension://hnemapnmnkccfommbacamppclohhcbfn/"]$\r$\n}$\r$\n'
    FileClose $1
    SetRegView 64
    WriteRegStr HKLM "Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser" "" "$INSTDIR\com.fluxdm.browser.json"
    WriteRegStr HKLM "Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser" "" "$INSTDIR\com.fluxdm.browser.json"

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME} Browser Extension Setup.lnk" "$WINDIR\explorer.exe" '"$INSTDIR\browser-extension\install.html"'
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section /o "Start FluxDM when I sign in" SEC_AUTOSTART
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${INFO_PRODUCTNAME}" '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    # Ensure the desktop executable and its WebView2 children do not keep files locked.
    # taskkill returns a non-zero status when FluxDM is not running; that is safe to ignore.
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /T /IM "${PRODUCT_EXECUTABLE}"'
    Pop $0
    Pop $1
    Sleep 500

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    SetRegView 64
    DeleteRegKey HKLM "Software\Google\Chrome\NativeMessagingHosts\com.fluxdm.browser"
    DeleteRegKey HKLM "Software\Microsoft\Edge\NativeMessagingHosts\com.fluxdm.browser"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${INFO_PRODUCTNAME}"

    ${If} $UninstallRemoveData == ${BST_CHECKED}
        Delete "$AppData\FluxDM\fluxdm.db"
        Delete "$AppData\FluxDM\fluxdm.db-shm"
        Delete "$AppData\FluxDM\fluxdm.db-wal"
        Delete "$AppData\FluxDM\fluxdm.db-journal"
        Delete "$AppData\FluxDM\fluxdm.db.corrupt.*"
        Delete "$AppData\FluxDM\fluxdm.log"
        Delete "$AppData\FluxDM\browser-bridge.json"
        # Only remove the directory if it is now empty. Unknown files and downloads survive.
        RMDir "$AppData\FluxDM"
    ${EndIf}

    # Program files and integration are always removed.
    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME} Browser Extension Setup.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
