//Copyright 2017-2019 Baidu Inc.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http: //www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package es

import (
	"github.com/olivere/elastic"
	"time"
	"context"
	"github.com/astaxie/beego/logs"
	"strconv"
	"github.com/astaxie/beego"
	"rasp-cloud/tools"
	"fmt"
	"strings"
	"rasp-cloud/conf"
	"errors"
	"rasp-cloud/environment"
	"reflect"
)

var (
	ElasticClient *elastic.Client
	Version       string
	ttlIndexes    = make(chan map[string]time.Duration, 1)
	minEsVersion  = "5.6.0"
	maxEsVersion  = "7.0.0"
)

func init() {
	ttlIndexes <- make(map[string]time.Duration)
	if *conf.AppConfig.Flag.StartType != conf.StartTypeReset {
		esAddr := conf.AppConfig.EsAddr
		client, err := elastic.NewSimpleClient(elastic.SetURL(esAddr...),
			elastic.SetBasicAuth(conf.AppConfig.EsUser, conf.AppConfig.EsPwd),
			elastic.SetSnifferTimeoutStartup(5*time.Second),
			elastic.SetSnifferTimeout(5*time.Second),
			elastic.SetSnifferInterval(30*time.Minute))
		if err != nil {
			tools.Panic(tools.ErrCodeESInitFailed, "init ES failed", err)
		}
		go startTTL(24 * time.Hour)

		Version, err = client.ElasticsearchVersion(esAddr[0])
		if err != nil {
			tools.Panic(tools.ErrCodeESInitFailed, "failed to get es version", err)
		}
		beego.Info("ES version: " + Version)
		if strings.Compare(Version, minEsVersion) < 0 {
			tools.Panic(tools.ErrCodeESInitFailed, "unable to support the ElasticSearch with a version lower than "+
				minEsVersion+ ","+ " the current version is "+ Version, nil)
		}
		if strings.Compare(Version, maxEsVersion) >= 0 {
			tools.Panic(tools.ErrCodeESInitFailed,
				"unable to support the ElasticSearch with a version greater than or equal to "+
					maxEsVersion+ ","+ " the current version is "+ Version, nil)
		}
		ElasticClient = client
	}
}

func startTTL(duration time.Duration) {
	ticker := time.NewTicker(duration)
	for {
		select {
		case <-ticker.C:
			DeleteExpiredData()
		}
	}
}

func DeleteExpiredData() {
	defer func() {
		if r := recover(); r != nil {
			beego.Error(r)
		}
	}()
	ttls := <-ttlIndexes
	defer func() {
		ttlIndexes <- ttls
	}()
	for index, duration := range ttls {
		expiredTime := strconv.FormatInt((time.Now().UnixNano()-int64(duration))/1000000, 10)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
		r, err := ElasticClient.DeleteByQuery(index).QueryString("@timestamp:<" + expiredTime).Do(ctx)
		if err != nil {
			if r != nil && r.Failures != nil {
				beego.Error(r.Failures)
			}
			beego.Error("failed to delete expired data for index " + index + ": " + err.Error())
		} else {
			var deleteNum int64
			if r != nil {
				deleteNum = r.Deleted
			}
			beego.Info("delete expired data successfully for index " + index + ", total: " +
				strconv.FormatInt(deleteNum, 10))
		}
		cancel()
	}
}

func DeleteLogs(index string) (err error) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()
	expiredTime := strconv.FormatInt((time.Now().UnixNano())/1000000, 10)
	//r, err := ElasticClient.Delete().Index(index).Type(docType).Id("*").Do(ctx)
	r, err := ElasticClient.DeleteByQuery(index).QueryString("@timestamp:<" + expiredTime).Do(ctx)
	if err != nil {
		if r != nil && r.Failures != nil {
			beego.Error(r.Failures)
		}
		beego.Error("failed to delete expired data for index " + index + ": " + err.Error())
	} else {
		var deleteNum int64
		if r != nil {
			deleteNum = r.Deleted
		}
		beego.Info("delete expired data successfully for index " + index + ", total: " +
			strconv.FormatInt(deleteNum, 10))
	}
	return err
}

func RegisterTTL(duration time.Duration, index string) {
	ttls := <-ttlIndexes
	defer func() {
		ttlIndexes <- ttls
	}()
	ttls[index] = duration
}

func CreateTemplate(name string, body string) (map[string]*elastic.IndicesGetTemplateResponse, bool, error) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()
	templateMappingOld, err := ElasticClient.IndexGetTemplate(name).Do(ctx)

	_, err = elastic.NewIndicesPutTemplateService(ElasticClient).Name(name).BodyString(body).Do(ctx)
	if err != nil {
		return templateMappingOld, true, err
	}
	templateMappingNew, err := ElasticClient.IndexGetTemplate(name).Do(ctx)
	if err != nil {
		e := RollbackTemplate(name, OldTemplateBackUp[name])
		if e != nil {
			beego.Error(e)
		}
		return templateMappingOld, true, err
	}

	beego.Info("put es template: " + name)
	if !reflect.DeepEqual(templateMappingOld, templateMappingNew) && len(templateMappingOld) != 0{
		beego.Info("template:", name, "has changed! Ready to Update Index Now!")
		return templateMappingOld, false, nil
	}
	return templateMappingOld, true, nil
}

func CreateEsIndex(index string, alias string, template string, flag bool) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()
	exists, err := ElasticClient.IndexExists(index).Do(ctx)
	if err != nil {
		return err
	}
	if !exists {
		createResult, err := ElasticClient.CreateIndex(index).Do(ctx)
		if err != nil {
			return err
		}
		logs.Info("create es index: " + createResult.Index)
		if err != nil {
			beego.Error("failed to create index with name " + index + ": " + err.Error())
			return err
		}
	} else {
		if UpdateMappingConfig[template] != nil && flag{
			beego.Info("updating template name:", template, "alias:", alias)
			err := UpdateMapping(index, alias, template, ctx)
			if err != nil {
				e := RollbackTemplate(template, OldTemplateBackUp[template])
				if e != nil {
					beego.Error(e)
				}
				return err
			}
		}
	}
	return nil
}

func Insert(index string, docType string, doc interface{}) (err error) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()
	_, err = ElasticClient.Index().Index(index).Type(docType).BodyJson(doc).Do(ctx)
	return
}

func BulkInsert(docType string, docs []map[string]interface{}) (err error) {
	bulkService := ElasticClient.Bulk()
	for _, doc := range docs {
		if doc["app_id"] == nil {
			beego.Error("failed to get app_id param from alarm: " + fmt.Sprintf("%+v", doc))
		}
		if appId, ok := doc["app_id"].(string); ok {
			if docType == "policy-alarm" || docType == "error-alarm" || docType == "attack-alarm" {
				fmt.Println(doc["upsert_id"])
				bulkService.Add(elastic.NewBulkUpdateRequest().
					Index("real-openrasp-" + docType + "-" + appId).
					Type(docType).
					Id(fmt.Sprint(doc["upsert_id"])).
					DocAsUpsert(true).
					Doc(doc))
			} else {
				if appId, ok := doc["app_id"].(string); ok {
					bulkService.Add(elastic.NewBulkIndexRequest().
						Index("real-openrasp-" + docType + "-" + appId).
						Type(docType).
						OpType("index").
						Doc(doc))
				}
			}
		} else {
			beego.Error("the type of alarm's app_id param is not string: " + fmt.Sprintf("%+v", doc))
		}
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()
	response, err := bulkService.Do(ctx)
	if response.Errors {
		return errors.New("ES bulk has errors: " + fmt.Sprintf("%+v", response.Failed()))
	}
	return err
}

func UpdateMapping (index string, alias string, template string, ctx context.Context) error {
	res, err := ElasticClient.Aliases().Index(alias).Do(ctx)
	if err != nil {
		e := RollbackTemplate(template, OldTemplateBackUp[template])
		if e != nil {
			beego.Error(e)
		}
		return err
	}
	if len(res.IndicesByAlias(alias)) != 1 {
		e := RollbackTemplate(template, OldTemplateBackUp[template])
		if e != nil {
			beego.Error(e)
		}
		return errors.New("find duplicate alias:" + fmt.Sprintf("%s", res))
	}

	oldIndexFromEs := res.IndicesByAlias(alias)[0]
	versionToEs := strings.Replace(environment.Version, ".", "", -1)
	newIndex := index + "-vidx-" + versionToEs
	oldIndexSplit := strings.Split(oldIndexFromEs, "-")
	if strings.Index(oldIndexFromEs, "-vidx") == -1 || oldIndexSplit[len(oldIndexSplit) - 2] != versionToEs {
		newIndex += "-0"
	} else if len(oldIndexSplit[len(oldIndexSplit) - 1]) == 1 {
		atoi, err := strconv.Atoi(oldIndexSplit[len(oldIndexSplit) - 1])
		if err != nil {
			return err
		}
		atoi += 1
		newIndex = newIndex + "-" + strconv.Itoa(atoi)
	}
	beego.Info("newIndex:", newIndex)
	err = CreateEsIndex(newIndex, alias, template, false)
	if err != nil {
		e := RollbackTemplate(template, OldTemplateBackUp[template])
		if e != nil {
			beego.Error(e)
		}
		return err
	}
	res, err = ElasticClient.Aliases().Index(newIndex).Do(ctx)
	if err != nil {
		e := RollbackTemplate(template, OldTemplateBackUp[template])
		if e != nil {
			beego.Error(e)
		}
		return err
	}

	aliases := ElasticClient.Alias()
	beego.Info("res.Indices:", res.Indices)
	beego.Info("res.Indices[newIndex]:", res.Indices[newIndex])
	beego.Info("Aliases[0]:", res.Indices[newIndex].Aliases[0])
	beego.Info("AliasName:", res.Indices[newIndex].Aliases[0].AliasName)
	// 去掉新建索引中的模版索引，换成原索引的模版索引
	aliasName := res.Indices[newIndex].Aliases[0].AliasName
	_, err = aliases.Add(newIndex, alias).Remove(newIndex, aliasName).Do(ctx)
	if err != nil {
		e := RollbackTemplate(template, OldTemplateBackUp[template])
		if e != nil {
			beego.Error(e)
		}
		return err
	}
	_, err = ElasticClient.Reindex().SourceIndex(index).DestinationIndex(newIndex).ProceedOnVersionConflict().Do(ctx)
	if err != nil {
		e := RollbackTemplate(template, OldTemplateBackUp[template])
		if e != nil {
			beego.Error(e)
		}
		return err
	}

	_, err = ElasticClient.Alias().Add(newIndex, alias).Remove(oldIndexFromEs, alias).Do(ctx)
	if err != nil {
		e := RollbackTemplate(template, OldTemplateBackUp[template])
		if e != nil {
			beego.Error(e)
		}
		return err
	}
	return nil
}

func RollbackTemplate(name string, body interface{}) error {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
	defer cancel()
	beego.Info("recover old env for template!")
	_, err := elastic.NewIndicesPutTemplateService(ElasticClient).Name(name).BodyJson(body).Do(ctx)
	if err != nil {
		return err
	}
	return nil
}