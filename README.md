# workflow
这个工程是为了组织复杂任务的运行. 这些任务通常需要分多步运行。

![](https://github.com/ZebraKK/workflow/workflows/Build/badge.svg)

task
    包含一组step
    这一组串行执行,则为serialTask
    这一组并行执行,则为parallelTask
    若有更加复杂的任务，即既有串行又有并行，那么应该使用多个task 再组合成更大的task

step
    最小的运行单元


任务流可以
    中断
    重试
    跳过
    回滚