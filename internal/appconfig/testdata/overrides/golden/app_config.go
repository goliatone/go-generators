// Code generated by app-config; DO NOT EDIT.

package appconfig

import "io/fs"

type Config struct {
	Database Database `json:"database" koanf:"database"`
}

type Database struct {
	AssetsFS fs.FS  `json:"assets_fs" koanf:"assets_fs"`
	DSN      string `json:"dsn" koanf:"dsn"`
}

func (d *Database) SetAssetsFS(val fs.FS) {
	d.AssetsFS = val
}
