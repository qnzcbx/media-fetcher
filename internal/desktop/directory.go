package desktop

import (
 "errors"
 "os"
 "path/filepath"
 "strings"
 "sync"

 "github.com/egoist/mygo"
)

var directoryDialogMu sync.Mutex

func fallbackDownloadDirectory(home func()(string,error))(string,error) {
 path,err:=home()
 if err!=nil || !filepath.IsAbs(path){return "",errors.New("无法获取系统下载目录")}
 return filepath.Join(path,"Downloads"),nil
}

func (s *DesktopService) GetDefaultDownloadDirectory()(string,error) {
 path,err:=systemDownloadDirectory()
 if err==nil && filepath.IsAbs(path){return filepath.Clean(path),nil}
 return fallbackDownloadDirectory(os.UserHomeDir)
}

func (s *DesktopService) ValidateDownloadDirectory(path string)(string,error) {
 if path=="" {
  var err error
  path,err=s.GetDefaultDownloadDirectory()
  if err!=nil{return "",err}
 }
 if !filepath.IsAbs(path) || strings.ContainsRune(path,0){return "",errors.New("下载位置必须是有效的绝对路径")}
 path=filepath.Clean(path)
 info,err:=os.Stat(path)
 if err!=nil{return "",errors.New("下载目录不存在或磁盘不可用，请在设置中重新选择")}
 if !info.IsDir(){return "",errors.New("下载位置不是文件夹，请重新选择")}
 f,err:=os.CreateTemp(path,".media-fetcher-check-*")
 if err!=nil{return "",errors.New("当前下载目录不可写入，请更换保存位置")}
 closeErr:=f.Close()
 removeErr:=os.Remove(f.Name())
 if closeErr!=nil || removeErr!=nil{return "",errors.New("无法验证下载目录的写入权限，请更换保存位置")}
 return path,nil
}

func (s *DesktopService) SelectDownloadDirectory(current string)(string,error) {
 if !directoryDialogMu.TryLock(){return "",errors.New("文件夹选择窗口已打开")}
 defer directoryDialogMu.Unlock()
 if !filepath.IsAbs(current){current,_=s.GetDefaultDownloadDirectory()}
 paths,err:=mygo.Dialog.Open(mygo.OpenDialogOptions{
  Parent:s.win,Title:"选择下载位置",DefaultPath:current,Directory:true,
 })
 if err!=nil{return "",errors.New("无法打开文件夹选择窗口")}
 if len(paths)==0{return "",nil}
 return s.ValidateDownloadDirectory(paths[0])
}

func (s *DesktopService) OpenDownloadDirectory(path string) error {
 validated,err:=s.ValidateDownloadDirectory(path)
 if err!=nil{return err}
 if err=mygo.Shell.OpenPath(validated);err!=nil{return errors.New("无法打开下载目录")}
 return nil
}
