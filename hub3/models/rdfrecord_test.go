package models

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RDFRecord", func() {

	hubID := "test_spec_123"
	spec := "spec"
	Context("When creating a new RDFRecord", func() {
		It("should not be empty", func() {
			record := NewRDFRecord(
				hubID,
				spec,
			)
			Expect(record.HubID).ToNot(BeEmpty())
		})
	})

	Context("when saving an RDFRecord", func() {
		It("should store the record in BoltDB", func() {
			record := NewRDFRecord(hubID, spec)
			err := record.Save()
			Expect(err).ToNot(HaveOccurred())
			var response RDFRecord
			err = orm.One("HubID", record.HubID, &response)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Given an HubID", func() {
		record := RDFRecord{
			HubID: hubID,
		}
		orgID, spec, localID, err := record.ExtractHubID()

		It("should provide access to the localID", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(localID).To(Equal("123"))
		})

		It("should provide access to the OrgID", func() {
			Expect(orgID).To(Equal("test"))
		})

		It("should provide access to the spec", func() {
			Expect(spec).To(Equal("spec"))
		})

	})

	Context("Given an illegal HubID", func() {
		record := RDFRecord{
			HubID: "testspec_123",
		}
		orgID, _, _, err := record.ExtractHubID()

		It("should return an error", func() {
			Expect(orgID).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})

	})

})
