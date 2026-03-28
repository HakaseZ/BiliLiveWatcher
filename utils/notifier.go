// 有两种处理方式
// A： 初始化一个MessageHandler，将notificer的Args传入，并等待接受Watcher的消息，触发后调用Notifier
// B： 分别初始化MessageHandler和notificer，解耦功能，但是会造成逻辑更复杂

package utils

func Notifier(int, int) {

}

func MessageHandler()
