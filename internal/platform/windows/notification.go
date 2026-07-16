package windows

import (
	"fmt"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
)

const notificationAppID = "FluxDM"

func ConfigureNotifications(executablePath, iconPath string) error {
	if executablePath == "" {
		return fmt.Errorf("notification executable path is required")
	}
	return toast.SetAppData(toast.AppData{
		AppID: notificationAppID, GUID: "3F84FEA7-595C-4C48-901B-A64F056D47B5",
		ActivationExe: executablePath, IconPath: iconPath,
	})
}

func NotifyDownloadComplete(fileName string) error {
	if fileName == "" {
		fileName = "Your download"
	}
	notification := toast.Notification{
		AppID: notificationAppID, Title: "Download complete", Body: fileName,
		Audio: toast.Default, Duration: toast.Short,
	}
	return notification.Push()
}
