# 模型定义
> main 表示主配置表, 

因为模型是代码重复率极高的部分，因此需要将其设计得更易于复制并且稍微改动一下即可适用于多种不同的数据
因此将模型定义规则如下

- cxx 以c开头表示collections 便于在golang model导入后需要修改名字
- model.go 定义数据库表需要的字段及表的名称
- method.go 定义操作数据的基本方法，根据rest api的规则命名
    - getList	GET http://my.api.url/posts?sort=["title","ASC"]&range=[0, 24]&filter={"title":"bar"}
    - getOne	GET http://my.api.url/posts/123
    - getMany	GET http://my.api.url/posts?filter={"ids":[123,456,789]}
    - getManyReference	GET http://my.api.url/posts?filter={"author_id":345}
    - create	POST http://my.api.url/posts
    - update	PUT http://my.api.url/posts/123
    - updateMany	Multiple calls to PUT http://my.api.url/posts/123
    - delete	DELETE http://my.api.url/posts/123
    - deleteMany	Multiple calls to DELETE http://my.api.url/posts/123

## 用户如何使用

导入

```

```