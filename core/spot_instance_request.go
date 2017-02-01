package autospotting

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type spotInstanceRequest struct {
	*ec2.SpotInstanceRequest
	region *region
	asg    *autoScalingGroup
}

// This function returns an Instance ID
func (s *spotInstanceRequest) waitForAndTagSpotInstance() {

	logger.Println(s.asg.name, "Waiting for spot instance for spot instance request",
		*s.SpotInstanceRequestId)

	ec2Client := s.region.services.ec2

	params := ec2.DescribeSpotInstanceRequestsInput{
		SpotInstanceRequestIds: []*string{s.SpotInstanceRequestId},
	}

	err := ec2Client.WaitUntilSpotInstanceRequestFulfilled(&params)
	if err != nil {
		logger.Println(s.asg.name, "Error waiting for instance:", err.Error())
		return
	}

	logger.Println(s.asg.name, "Done waiting for an instance.")

	// Now we try to get the InstanceID of the instance we got
	requestDetails, err := ec2Client.DescribeSpotInstanceRequests(&params)
	if err != nil {
		logger.Println(s.asg.name, "Failed to describe spot instance requests")
	}

	// due to the waiter we can now safely assume all this data is available
	spotInstanceID := requestDetails.SpotInstanceRequests[0].InstanceId

	logger.Println(s.asg.name, "found new spot instance", *spotInstanceID,
		"\nTagging it to match the other instances from the group")

	// we need to re-scan in order to have the information a
	s.region.scanInstances()

	tags := s.asg.propagatedInstanceTags()

	thisInstance := s.region.instances.get(*spotInstanceID)
	if thisInstance != nil {
		thisInstance.tag(tags)
		logger.Println("Tagged instance", *spotInstanceID)
	} else {
		logger.Println("Failed to tag instance", *spotInstanceID)
	}
}

func (s *spotInstanceRequest) tag(asgName string) {
	svc := s.region.services.ec2

	_, err := svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{s.SpotInstanceRequestId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("launched-for-asg"),
				Value: aws.String(asgName),
			},
		},
	})

	if err != nil {
		// Print the error, cast err to awserr.Error to get the Code and
		// Message from an error.
		logger.Println(asgName,
			"Failed to create tags for the spot instance request",
			err.Error())
		return
	}

	logger.Println(asgName, "successfully tagged spot instance request",
		*s.SpotInstanceRequestId)
}
