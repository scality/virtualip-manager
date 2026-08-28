package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/scality/virtualip-manager/pkg/domain"
)

var _ = Describe("Parse Input Data", func() {
	var (
		testResource *domain.VirtualIPConfig
		err          error
	)

	Context("When loading input file", func() {
		It("should successfully load a complete input file", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputComplete)
			Expect(err).NotTo(HaveOccurred())
			Expect(domain.SUPPORTED_API_VERSION).To(ContainElement(*testResource.ApiVersion))
			Expect(*testResource.Kind).To(Equal(domain.EXPECTED_KIND))
			Expect(testResource.Addresses).NotTo(BeEmpty())
			Expect(testResource.Healthcheck).NotTo(BeNil())
		})

		It("should successfully load a input file with missing healthcheck", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputNoHealthcheck)
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource.Healthcheck).To(BeNil())
		})

		It("should successfully load a input file with empty healthcheck", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputEmptyHealthcheck)
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource.Healthcheck).To(BeNil())
		})

		It("should successfully load a input file with only a healthcheckNodePort", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputNodePortOnly)
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource.Healthcheck).To(BeNil())
			Expect(testResource.HealthCheckNodePort).NotTo(BeNil())
			Expect(*testResource.HealthCheckNodePort).To(Equal("http://localhost:31846"))
		})

		It("should successfully load a input file with both healthchecks", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputBothHealthchecks)
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource.Healthcheck).NotTo(BeNil())
			Expect(testResource.HealthCheckNodePort).NotTo(BeNil())
		})

		It("should successfully load a input file with empty healthcheckNodePort", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputEmptyNodePort)
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource.HealthCheckNodePort).To(BeNil())
		})

		It("should return an error if the input file is a YAML file but malformed", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputMalformed)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file has empty addresses", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputEmptyAddresses)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file has missing addresses", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputMissingAddresses)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file is missing the Kind", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputMissingKind)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file is the wrong Kind", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputWrongKind)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file is missing the apiVersion", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputMissingApiVersion)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file is the wrong apiVersion", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputWrongApiVersion)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file is empty", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputEmpty)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})

		It("should return an error if the input file is a comment only", func() {
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputCommentOnly)
			Expect(err).To(HaveOccurred())
			Expect(testResource).To(BeNil())
		})
	})
})

var _ = Describe("Generate Output Configuration", func() {
	var (
		testResource *domain.VirtualIPConfig
		err          error
	)

	Context("When generating output configuration", func() {
		It("should successfully generate output configuration with a complete input file", func() {
			By("loading the input file")
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputComplete)
			Expect(err).NotTo(HaveOccurred())
			Expect(domain.SUPPORTED_API_VERSION).To(ContainElement(*testResource.ApiVersion))
			Expect(*testResource.Kind).To(Equal(domain.EXPECTED_KIND))
			Expect(testResource.Addresses).NotTo(BeEmpty())
			Expect(testResource.Healthcheck).NotTo(BeNil())

			By("generating the output configuration")
			outputData, err := testingSuite.container.GetConfigGenerator().GenerateConfiguration(testResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputData).To(Equal(outputComplete))
		})

		It("should successfully generate output configuration with an input file with missing healthcheck", func() {
			By("loading the input file")
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputNoHealthcheck)
			Expect(err).NotTo(HaveOccurred())
			Expect(domain.SUPPORTED_API_VERSION).To(ContainElement(*testResource.ApiVersion))
			Expect(*testResource.Kind).To(Equal(domain.EXPECTED_KIND))
			Expect(testResource.Addresses).NotTo(BeEmpty())
			Expect(testResource.Healthcheck).To(BeNil())

			By("generating the output configuration")
			outputData, err := testingSuite.container.GetConfigGenerator().GenerateConfiguration(testResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputData).To(Equal(outputNoHealthcheck))
		})

		It("should generate a check_get_nodeport script when only healthcheckNodePort is set", func() {
			By("loading the input file")
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputNodePortOnly)
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource.Healthcheck).To(BeNil())
			Expect(testResource.HealthCheckNodePort).NotTo(BeNil())

			By("generating the output configuration")
			outputData, err := testingSuite.container.GetConfigGenerator().GenerateConfiguration(testResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputData).To(Equal(outputNodePortOnly))
		})

		It("should generate both scripts and track both when the two healthchecks are set", func() {
			By("loading the input file")
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputBothHealthchecks)
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource.Healthcheck).NotTo(BeNil())
			Expect(testResource.HealthCheckNodePort).NotTo(BeNil())

			By("generating the output configuration")
			outputData, err := testingSuite.container.GetConfigGenerator().GenerateConfiguration(testResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputData).To(Equal(outputBothHealthchecks))
		})

		It("should not generate any track_script section when no healthcheck is set", func() {
			By("loading the input file")
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputNoHealthcheck)
			Expect(err).NotTo(HaveOccurred())

			By("generating the output configuration")
			outputData, err := testingSuite.container.GetConfigGenerator().GenerateConfiguration(testResource)
			Expect(err).NotTo(HaveOccurred())
			Expect(outputData).NotTo(ContainSubstring("track_script"))
			Expect(outputData).NotTo(ContainSubstring("vrrp_script"))
		})

		It("should return an error if no interfaces are found", func() {
			By("loading the input file")
			testResource, err = testingSuite.container.GetConfigGenerator().ParseInputData(inputNoMatchingInterfaces)
			// No error is returned because the input file is valid
			Expect(err).NotTo(HaveOccurred())
			Expect(testResource).NotTo(BeNil())

			By("generating the output configuration")
			outputData, err := testingSuite.container.GetConfigGenerator().GenerateConfiguration(testResource)
			Expect(err).To(HaveOccurred())
			Expect(outputData).To(BeEmpty())
		})
	})
})
