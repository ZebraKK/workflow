# workflow
接手的业务服务出发，按自己理解也实现一下

![](https://github.com/ZebraKK/workflow/workflows/Build/badge.svg)

task
    包含一组step
    这一组串行执行,则为serialTask
    这一组并行执行,则为parallelTask
    若有更加复杂的任务，即即有串行又有并行，那么应该使用多个task 再组合成更大的task

step
    最小的运行单元