rpc

为了兼容现有的一些项目, 仿照go标准库的rpc而实现了一套新的rpc框架,

跟标准库的rpc相比, 主要有下面几个区别:

1. seq使用的是uint32, 而不是uint64
2. 不使用string类型的method name来寻找被调用函数, 而使用uint32类型的cmd
3. 不返回string类型的Error, 使用uint32作为error code
4. 更好的支持有状态的连接
5. 被调函数不必是某个type的method
