# Telescope DNS
* 支持按域名后缀匹配/GFWList匹配/半智能污染检测进行分组
* 支持多hosts文件 + 手动指定hosts
* DNS查询支持socks5代理
* DNS记录缓存

## 整体结构
  ![](images/arch.png)
## 分组机制
  ![](images/grouping.png)

## 使用说明

* 在[release页](https://github.com/wolf-joe/ts-dns/releases)下载软件包；
* 解压后直接运行进程（使用默认配置，不推荐），或编辑自己的配置文件后运行进程：
  ```shell
  # ./ts-dns -c ts-dns.toml
  ./ts-dns
  ```

## 配置说明


```toml
listen = ":53"

[groups]
  [groups.clean]
  dns = ["119.29.29.29", "223.5.5.5", "114.114.114.114"]

  [groups.dirty]
  dns = ["208.67.222.222:5353", "176.103.130.130:5353"]
  suffix = ["google.com", "twimg.com", "quoracdn.net"]
```

* 完整配置文件见`ts-dns.full.toml`。
* `gfwlist.txt`参考`https://github.com/gfwlist/gfwlist/raw/master/gfwlist.txt`。
* 反污染功能无法关闭。如不想使用该功能可将`clean`组和`dirty`组中的`dns`设为相同值。
* 当配置了`redis`时，本程序将使用`Redis`作为污染检测结果缓存，反之则使用内置的`TTLMap`作为缓存。
* 污染检测机制尚不完善，目前已知无法正确判定是否被污染的域名如上文`dirty`组里的`suffix`所示。推荐使用`GFWList`实现更好的准确度。
* `dirty`组DNS里的地址推荐设置为自建的`dnscrypt-proxy`（即搭配DOH/DOT使用）或使用`socks5`代理。

## TODO

* DNS响应择优
* 自动添加IPSET
* DNS over HTTPS/TLS/TCP等支持

## 特别鸣谢
* [github.com/arloan/prdns](https://github.com/arloan/prdns)