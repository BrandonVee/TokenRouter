import batchImageGuide from './batchImageGuide'
import home from './home'
import keyUsage from './keyUsage'
import setup from './setup'

// 兼容旧引用：新代码应直接引用对应领域模块。
export default {
  ...batchImageGuide,
  ...home,
  ...keyUsage,
  ...setup,
}
