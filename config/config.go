package config

import (
	"MusicConvert/model"
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

var cfg = &Config{}

type Config struct {
	// IsSplit 整轨是否切割, cue 文件配合
	IsSplit model.SwitchState `yaml:"is_split"`
	// IsDelSrc 转换格式成功后, 是否删掉源文件
	IsDelSrc model.SwitchState `yaml:"is_del_src"`
	SrcPath  string            `yaml:"src_path"`
	// DescType 目标编码
	DescType string `yaml:"desc_type"`
	// DescPath 目标文件存储位置, 默认和源文件一致
	DescPath string `yaml:"desc_path"`
	// OutputRoot 输出目录
	OutputRoot string `yaml:"output_root"`
}

func GetDefaultConfig() *Config {
	return &Config{
		IsSplit:  model.OFF,
		IsDelSrc: model.OFF,
		SrcPath:  "./",
		DescType: "FLAC",
		DescPath: "",
	}
}

func Init(config string) *Config {

	if config == "" {
		return GetDefaultConfig()
	}
	//读取yaml文件到缓存中
	s, err := ioutil.ReadFile(config)
	if err != nil {
		fmt.Print(err)
	}

	//yaml文件内容影射到结构体中
	err = yaml.Unmarshal(s, cfg)
	if err != nil {
		fmt.Println("error")
		return nil
	}
	return cfg
}

func GetDescType() string {
	return cfg.DescType
}

func GetOutputRoot() string {
	return cfg.OutputRoot
}
func GetIsDelSrc() model.SwitchState {
	return cfg.IsDelSrc
}
