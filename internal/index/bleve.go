package index

import (
	"errors"
	"strconv"
	// "fmt"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/analysis/token/length"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	// "github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"

	bleveUnicode "github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/rs/zerolog/log"
)

type BleveIndexer struct {
	index bleve.Index
	path  string
}

func NewBleveIndexer(path string) *BleveIndexer {
	return &BleveIndexer{path: path}
}

func (i *BleveIndexer) Init() error {
	errChan := make(chan error, 1)

	go func() {
		bleveIndex, err := i.open()
		if err != nil {
			log.Error().Err(err).Msg("Failed to open Bleve index")
			i.Close()
			errChan <- err
			return
		}
		i.index = bleveIndex
		log.Info().Msg("Bleve indexer initialized")
		errChan <- nil
	}()

	return <-errChan
}

func (i *BleveIndexer) open() (bleve.Index, error) {
	bleveIndex, err := bleve.Open(i.path)
	if err == nil {
		return bleveIndex, nil
	}

	if !errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		return nil, err
	}

	// ==========================================
    // 1. 定义索引规则 (Mapping)
	mapping := bleve.NewIndexMapping()

	// 定义过滤器：去除长度小于 2 的无意义字符 (如 a, b, 1)
	if err = mapping.AddCustomTokenFilter("length_filter_min2", map[string]interface{}{
		"type": length.Name,
		"min":  2.0,
	}); err != nil {
		return nil, err
	}

	// 定义过滤器：Unicode 标准化
	if err = mapping.AddCustomTokenFilter("unicodeNormalize", map[string]any{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	}); err != nil {
		return nil, err
	}

	// --- 分析器 1: 【拆分模式】 (用于搜局部) 效果: "UserLogin" -> "user", "login" ---
	if err = mapping.AddCustomAnalyzer("code_split", map[string]interface{}{
		"type":      custom.Name,
		"tokenizer": bleveUnicode.Name,
		"token_filters": []string{
			"unicodeNormalize",
			camelcase.Name,       // 核心：拆分驼峰
			lowercase.Name,       // 转小写
			"length_filter_min2", // 去掉拆分后太短的
		},
	}); err != nil {
		return nil, err
	}

	// --- 分析器 2: 【精确模式】 (用于搜全词) 效果: "UserLogin" -> "userlogin" ---
	if err = mapping.AddCustomAnalyzer("code_exact", map[string]interface{}{
		"type":      custom.Name,
		"tokenizer": bleveUnicode.Name,
		"token_filters": []string{
			"unicodeNormalize",
			lowercase.Name,       // 只转小写，不拆分！
			"length_filter_min2",
		},
	}); err != nil {
		return nil, err
	}

	docMapping := bleve.NewDocumentMapping()
	// 数值字段
	docMapping.AddFieldMappingsAt("GistID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("UserID", bleve.NewNumericFieldMapping())
	docMapping.AddFieldMappingsAt("Visibility", bleve.NewNumericFieldMapping())

	// Metadata 字段 (标题、文件名等，通常适合拆分搜)
	metaMapping := bleve.NewTextFieldMapping()
	metaMapping.Analyzer = "code_split"
	docMapping.AddFieldMappingsAt("Username", metaMapping)
	docMapping.AddFieldMappingsAt("Title", metaMapping)
	docMapping.AddFieldMappingsAt("Filenames", metaMapping)
	docMapping.AddFieldMappingsAt("Extensions", metaMapping)
	docMapping.AddFieldMappingsAt("Languages", metaMapping)
	docMapping.AddFieldMappingsAt("Topics", metaMapping)


	// --- 核心 Content 字段的双重映射 ---
    
    // 映射 A: Content (精确匹配) 存: "userlogin"
	contentExact := bleve.NewTextFieldMapping()
	contentExact.Name = "Content" // 字段名
	contentExact.Analyzer = "code_exact"
	contentExact.Store = false
	contentExact.IncludeTermVectors = true

	// 映射 B: ContentSplit (拆分匹配) 存: "user", "login"
	contentSplit := bleve.NewTextFieldMapping()
	contentSplit.Name = "ContentSplit" // 虚拟字段名
	contentSplit.Analyzer = "code_split"
	contentSplit.Store = false
	contentSplit.IncludeTermVectors = true

	// 将同一个 Content 内容，同时塞进这两个映射里
	docMapping.AddFieldMappingsAt("Content", contentExact, contentSplit)
	mapping.DefaultMapping = docMapping
	return bleve.New(i.path, mapping)
}


func (i *BleveIndexer) Close() {
	if i == nil || i.index == nil {
		return
	}

	err := i.index.Close()
	if err != nil {
		log.Error().Err(err).Msg("Failed to close Bleve index")
	}
	log.Info().Msg("Bleve indexer closed")
}

func (i *BleveIndexer) Add(gist *Gist) error {
	if gist == nil {
		return errors.New("failed to add nil gist to index")
	}
	return (*atomicIndexer.Load()).(*BleveIndexer).index.Index(strconv.Itoa(int(gist.GistID)), gist)
}

func (i *BleveIndexer) Remove(gistID uint) error {
	return (*atomicIndexer.Load()).(*BleveIndexer).index.Delete(strconv.Itoa(int(gistID)))
}

func (i *BleveIndexer) Search(queryStr string, queryMetadata SearchGistMetadata, userId uint, page int) ([]uint, uint64, map[string]int, error) {
	var err error
	var indexerQuery query.Query
	
	// ==========================================
    // 3. 搜索逻辑 (同时搜两个字段)
	if queryStr != "" {
		queryStr = strings.ToLower(strings.TrimSpace(queryStr))

        // 查 Content (精确): 权重高，匹配 "userlogin"
		q1 := bleve.NewMatchQuery(queryStr)
		q1.SetField("Content")
		q1.SetBoost(1.5)		// ⚠️ 删掉了 Fuzziness=2，代码搜索不需要模糊

        // 查 ContentSplit (拆分): 权重低，匹配 "login"
		q2 := bleve.NewMatchQuery(queryStr)
		q2.SetField("ContentSplit")
		q2.SetBoost(1.0)

        // Metadata 查询
        titleQuery := bleve.NewMatchQuery(queryStr)
        titleQuery.SetField("Title")
        titleQuery.SetBoost(3.0)

        usernameQuery := bleve.NewMatchQuery(queryStr)
        usernameQuery.SetField("Username")
        usernameQuery.SetBoost(2.0)
        
        filenameQuery := bleve.NewMatchQuery(queryStr)
        filenameQuery.SetField("Filenames")
        filenameQuery.SetBoost(2.5)

        // 只要满足任意一个即可 (Disjunction)
		indexerQuery = bleve.NewDisjunctionQuery(
            q1, 
            q2, 
            titleQuery,
            usernameQuery, 
            filenameQuery,
        )
	} else {
		contentQuery := bleve.NewMatchAllQuery()
		indexerQuery = contentQuery
	}

	// 权限过滤
	visibilityZero := float64(0)
	truee := true
	publicQuery := bleve.NewNumericRangeInclusiveQuery(&visibilityZero, &visibilityZero, &truee, &truee)
	publicQuery.SetField("Visibility")

	userIdMatch := float64(userId)
	userIdQuery := bleve.NewNumericRangeInclusiveQuery(&userIdMatch, &userIdMatch, &truee, &truee)
	userIdQuery.SetField("UserID")

	accessQuery := bleve.NewDisjunctionQuery(publicQuery, userIdQuery)
	indexerQuery = bleve.NewConjunctionQuery(accessQuery, indexerQuery)

	// 处理 All 和其他 Metadata
	if queryMetadata.All != "" {
		allQueries := make([]query.Query, 0)
		fields := []string{"Username", "Title", "Filenames", "Languages", "Topics"}
		for _, f := range fields {
			q := bleve.NewMatchQuery(queryMetadata.All) // 用 MatchQuery 以支持分词
			q.SetField(f)
			allQueries = append(allQueries, q)
		}
        // Extension 单独处理
        extQ := bleve.NewMatchQuery("." + queryMetadata.All)
        extQ.SetField("Extensions")
        allQueries = append(allQueries, extQ)

		allDisjunction := bleve.NewDisjunctionQuery(allQueries...)
		indexerQuery = bleve.NewConjunctionQuery(indexerQuery, allDisjunction)
	} else {
		addQuery := func(field, value string) {
			if value != "" && value != "." {
				q := bleve.NewMatchQuery(value) // 用 MatchQuery
				q.SetField(field)
				indexerQuery = bleve.NewConjunctionQuery(indexerQuery, q)
			}
		}
		addQuery("Username", queryMetadata.Username)
		addQuery("Title", queryMetadata.Title)
		addQuery("Extensions", "."+queryMetadata.Extension)
		addQuery("Filenames", queryMetadata.Filename)
		addQuery("Languages", queryMetadata.Language)
		addQuery("Topics", queryMetadata.Topic)
	}

	languageFacet := bleve.NewFacetRequest("Languages", 10)
	perPage := 10
	offset := (page - 1) * perPage

	s := bleve.NewSearchRequestOptions(indexerQuery, perPage+1, offset, false)
	s.AddFacet("languageFacet", languageFacet)
    
    // 返回这些字段以便调试
	s.Fields = []string{"GistID", "Title", "Username", "Filenames"}
	s.IncludeLocations = true // 开启位置匹配，方便调试

	results, err := (*atomicIndexer.Load()).(*BleveIndexer).index.Search(s)
	if err != nil {
		return nil, 0, nil, err
	}

	// ==========================================
    // 4. Debug 打印
    // if queryStr != "" {
    //     fmt.Println("\n================= 🔍 DEBUG SEARCH ================= ")
    //     fmt.Printf("关键词: [%s]  找到: %d 个\n", queryStr, results.Total)
        
    //     for i, hit := range results.Hits {
    //         title := hit.Fields["Title"]
    //         // 简单的打印，只显示匹配了哪些字段
    //         var matchedFields []string
    //         if hit.Locations != nil {
    //             for field := range hit.Locations {
    //                 matchedFields = append(matchedFields, field)
    //             }
    //         }
            
    //         fmt.Printf("#%d [ID:%s] Score:%.2f Title:%v 匹配字段:%v\n", 
    //             i+1, hit.ID, hit.Score, title, matchedFields)
    //     }
    //     fmt.Println("===================================================\n")
    // }


	gistIds := make([]uint, 0, len(results.Hits))
	for _, hit := range results.Hits {
		gistIds = append(gistIds, uint(hit.Fields["GistID"].(float64)))
	}

	languageCounts := make(map[string]int)
	if facets, found := results.Facets["languageFacet"]; found {
		for _, term := range facets.Terms.Terms() {
			languageCounts[term.Term] = term.Count
		}
	}

	return gistIds, results.Total, languageCounts, nil
}

