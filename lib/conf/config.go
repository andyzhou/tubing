package conf

import (
	"errors"
	"fmt"
	"sync"
)

/*
 * ini config parse face
 */

//face info
type Config struct {
	cfgRootPath string
	cfgMap map[string]*File //tag -> *File
	autoReload bool
	sync.RWMutex
}

//construct
func NewConfig() *Config {
	return NewConfigWithPara(".")
}

func NewConfigWithPara(cfgRootPath string) *Config {
	//self init
	this := &Config{
		cfgRootPath:cfgRootPath,
		cfgMap:make(map[string]*File),
	}
	return this
}

//get all section
func (f *Config) GetAllSection(tag string) map[string]map[string]string {
	if tag == "" || f.cfgMap == nil {
		return nil
	}
	v, ok := f.cfgMap[tag]
	if !ok {
		return nil
	}
	//init result
	result := make(map[string]map[string]string)
	for section, val := range v.GetAllSection() {
		result[section] = val
	}
	return result
}

//get section
func (f *Config) GetSection(tag, section string) map[string]string {
	oneCfg := f.GetOneConfig(tag)
	if oneCfg == nil {
		return nil
	}
	return oneCfg.Section(section)
}

//get all section of one config
func (f *Config) GetOneConfig(tag string) *File {
	//basic check
	if tag == "" || f.cfgMap == nil {
		return nil
	}

	//get with locker
	f.Lock()
	defer f.Unlock()
	v, ok := f.cfgMap[tag]
	if ok {
		return v
	}
	return nil
}

//load one config file
func (f *Config) LoadConfig(cfgFileName string) error {
	if cfgFileName == "" {
		return errors.New("invalid parameter")
	}

	//format config full path
	cfgFileFullPath := fmt.Sprintf("%s/%s", f.cfgRootPath, cfgFileName)

	//load
	file, err := LoadFile(cfgFileFullPath)
	if err != nil {
		return err
	}

	//sync into running map
	f.Lock()
	defer f.Unlock()
	f.cfgMap[cfgFileName] = &file

	return nil
}

//set auto reload switch
func (f *Config) SetAutoReload(switcher bool) {
	f.Lock()
	defer f.Unlock()
	f.autoReload = switcher
}