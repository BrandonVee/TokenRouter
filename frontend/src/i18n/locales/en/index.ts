import home from './home'
import keyUsage from './keyUsage'
import setup from './setup'
import batchImageGuide from './batchImageGuide'
import common from './common'
import dashboard from './dashboard'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import team from './team'
import imageHistory from './imageHistory'

import adminAccessKeys from './adminAccessKeys'

export default {
  adminAccessKeys,
  ...home,
  ...keyUsage,
  ...setup,
  ...batchImageGuide,
  ...common,
  ...dashboard,
  ...batchImage,
  admin,
  ...misc,
  ...team,
  ...imageHistory,
}
