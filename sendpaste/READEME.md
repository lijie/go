sendpaste

sendpaste是一个简单的云复制粘贴服务.目前可以复制文字和文件, 但还只能用于命令行.

服务端需要部署sendpasted

复制文字
sendpaste "helloworld"

复制文件
sendpaste -f a.txt

获取最近一次复制的内容
sendpaste

配置文件放在$(HOME)/.sendpaste.json
	{
		"ServerAddr": "ip:port",
		"Auth": ""
	}
