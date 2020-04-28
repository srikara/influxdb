// Libraries
import React, {FC, useEffect} from 'react'
import {connect} from 'react-redux'

// Components
import {
  Button,
  IconFont,
  ComponentColor,
  ComponentStatus,
} from '@influxdata/clockface'

// Actions
import {
  checkBucketLimits as checkBucketLimitsAction,
  LimitStatus,
} from 'src/cloud/actions/limits'
import {showOverlay, dismissOverlay} from 'src/overlays/actions/overlays'

// Utils
import {extractBucketLimits} from 'src/cloud/utils/limits'

// Types
import {AppState} from 'src/types'

interface StateProps {
  limitStatus: LimitStatus
}

interface DispatchProps {
  onShowOverlay: typeof showOverlay
  onDismissOverlay: typeof dismissOverlay
  checkBucketLimits: typeof checkBucketLimitsAction
}

interface OwnProps {}

type Props = OwnProps & StateProps & DispatchProps

const CreateBucketButton: FC<Props> = ({
  limitStatus,
  checkBucketLimits,
  onShowOverlay,
  onDismissOverlay,
}) => {
  useEffect(() => {
    // Check bucket limits when component mounts
    checkBucketLimits()
  }, [])

  const limitExceeded = limitStatus === LimitStatus.EXCEEDED
  const text = 'Create Bucket'
  let titleText = 'Click to create a bucket'
  let buttonStatus = ComponentStatus.Default

  if (limitExceeded) {
    titleText = 'This account has the maximum number of buckets allowed'
    buttonStatus = ComponentStatus.Disabled
  }

  const handleItemClick = (): void => {
    if (limitExceeded) {
      return
    }

    onShowOverlay('create-bucket', null, onDismissOverlay)
  }

  return (
    <Button
      icon={IconFont.Plus}
      color={ComponentColor.Primary}
      text={text}
      titleText={titleText}
      onClick={handleItemClick}
      testID="Create Bucket"
      status={buttonStatus}
    />
  )
}

const mstp = (state: AppState): StateProps => {
  return {
    limitStatus: extractBucketLimits(state.cloud.limits),
  }
}

const mdtp: DispatchProps = {
  onShowOverlay: showOverlay,
  onDismissOverlay: dismissOverlay,
  checkBucketLimits: checkBucketLimitsAction,
}

export default connect<StateProps, DispatchProps, OwnProps>(
  mstp,
  mdtp
)(CreateBucketButton)
