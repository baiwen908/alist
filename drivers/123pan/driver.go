package _23pan

import (
	"fmt"
	"github.com/Xhofe/alist/conf"
	"github.com/Xhofe/alist/drivers"
	"github.com/Xhofe/alist/model"
	"github.com/Xhofe/alist/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"path/filepath"
)

type Pan123 struct {}

var driverName = "123Pan"

func (driver Pan123) Items() []drivers.Item {
	return []drivers.Item{
		{
			Name:        "proxy",
			Label:       "proxy",
			Type:        "bool",
			Required:    true,
			Description: "allow proxy",
		},
		{
			Name:        "username",
			Label:       "username",
			Type:        "string",
			Required:    true,
			Description: "account username/phone number",
		},
		{
			Name:        "password",
			Label:       "password",
			Type:        "string",
			Required:    true,
			Description: "account password",
		},
		{
			Name:     "root_folder",
			Label:    "root folder file_id",
			Type:     "string",
			Required: false,
		},
		{
			Name:     "order_by",
			Label:    "order_by",
			Type:     "select",
			Values:   "name,fileId,updateAt,createAt",
			Required: true,
		},
		{
			Name:     "order_direction",
			Label:    "order_direction",
			Type:     "select",
			Values:   "asc,desc",
			Required: true,
		},
	}
}

func (driver Pan123) Save(account *model.Account, old *model.Account) error {
	if account.RootFolder == "" {
		account.RootFolder = "0"
	}
	err := driver.Login(account)
	return err
}

func (driver Pan123) File(path string, account *model.Account) (*model.File, error) {
	path = utils.ParsePath(path)
	if path == "/" {
		return &model.File{
			Id:        account.RootFolder,
			Name:      account.Name,
			Size:      0,
			Type:      conf.FOLDER,
			Driver:    driverName,
			UpdatedAt: account.UpdatedAt,
		}, nil
	}
	dir, name := filepath.Split(path)
	files, err := driver.Files(dir, account)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.Name == name {
			return &file, nil
		}
	}
	return nil, drivers.PathNotFound
}

func (driver Pan123) Files(path string, account *model.Account) ([]model.File, error) {
	path = utils.ParsePath(path)
	var rawFiles []Pan123File
	cache, err := conf.Cache.Get(conf.Ctx, fmt.Sprintf("%s%s", account.Name, path))
	if err == nil {
		rawFiles, _ = cache.([]Pan123File)
	} else {
		file, err := driver.File(path, account)
		if err != nil {
			return nil, err
		}
		rawFiles, err = driver.GetFiles(file.Id, account)
		if err != nil {
			return nil, err
		}
		if len(rawFiles) > 0 {
			_ = conf.Cache.Set(conf.Ctx, fmt.Sprintf("%s%s", account.Name, path), rawFiles, nil)
		}
	}
	files := make([]model.File, 0)
	for _, file := range rawFiles {
		files = append(files, *driver.FormatFile(&file))
	}
	return files, nil
}

func (driver Pan123) Link(path string, account *model.Account) (string, error) {
	file, err := driver.GetFile(utils.ParsePath(path), account)
	if err != nil {
		return "", err
	}
	var resp Pan123DownResp
	_, err = pan123Client.R().SetResult(&resp).SetHeader("authorization", "Bearer "+account.AccessToken).
		SetBody(drivers.Json{
			"driveId":   0,
			"etag":      file.Etag,
			"fileId":    file.FileId,
			"fileName":  file.FileName,
			"s3keyFlag": file.S3KeyFlag,
			"size":      file.Size,
			"type":      file.Type,
		}).Post("https://www.123pan.com/api/file/download_info")
	if err != nil {
		return "", err
	}
	if resp.Code != 0 {
		if resp.Code == 401 {
			err := driver.Login(account)
			if err != nil {
				return "", err
			}
			return driver.Link(path, account)
		}
		return "", fmt.Errorf(resp.Message)
	}
	return resp.Data.DownloadUrl, nil
}

func (driver Pan123) Path(path string, account *model.Account) (*model.File, []model.File, error) {
	path = utils.ParsePath(path)
	log.Debugf("pan123 path: %s", path)
	file, err := driver.File(path, account)
	if err != nil {
		return nil, nil, err
	}
	if file.Type != conf.FOLDER {
		file.Url, _ = driver.Link(path, account)
		return file, nil, nil
	}
	files, err := driver.Files(path, account)
	if err != nil {
		return nil, nil, err
	}
	return nil, files, nil
}

func (driver Pan123) Proxy(c *gin.Context, account *model.Account) {
	c.Request.Header.Del("origin")
}

func (driver Pan123) Preview(path string, account *model.Account) (interface{}, error) {
	return nil, nil
}

var _ drivers.Driver = (*Pan123)(nil)