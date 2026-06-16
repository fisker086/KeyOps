package bill

import "testing"

func TestIsExcludedFromServiceTotal(t *testing.T) {
	payer := "995362308517"

	tests := []struct {
		name        string
		payer       string
		usage       string
		serviceType string
		serviceCode string
		want        bool
	}{
		{
			name:        "payer enterprise support",
			payer:       payer,
			usage:       payer,
			serviceType: "AWS Support (Enterprise)",
			want:        true,
		},
		{
			name:        "payer shield stays in service total",
			payer:       payer,
			usage:       payer,
			serviceType: "AWS Shield",
			want:        false,
		},
		{
			name:        "linked account shield stays",
			payer:       payer,
			usage:       "862699281247",
			serviceType: "AWS Shield",
			want:        false,
		},
		{
			name:        "linked account ec2 stays",
			payer:       payer,
			usage:       "353805302579",
			serviceType: "Amazon Elastic Compute Cloud",
			want:        false,
		},
		{
			name:        "payer s3 stays",
			payer:       payer,
			usage:       payer,
			serviceType: "Amazon Simple Storage Service",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExcludedFromServiceTotal(tt.payer, tt.usage, tt.serviceType, tt.serviceCode)
			if got != tt.want {
				t.Fatalf("isExcludedFromServiceTotal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsYCloudMarketplaceCost(t *testing.T) {
	tests := []struct {
		name            string
		usage           string
		serviceType     string
		billingEntity   string
		wantMarketplace bool
	}{
		{
			name:            "cur marketplace entity",
			usage:           "014498665437",
			serviceType:     "Some MP Product",
			billingEntity:   "AWS Marketplace",
			wantMarketplace: true,
		},
		{
			name:            "log glacier deep archive",
			usage:           "689860182975",
			serviceType:     "Amazon S3 Glacier Deep Archive",
			billingEntity:   "AWS",
			wantMarketplace: true,
		},
		{
			name:            "lark glacier stays aws",
			usage:           "484333365687",
			serviceType:     "Amazon S3 Glacier Deep Archive",
			billingEntity:   "AWS",
			wantMarketplace: false,
		},
		{
			name:            "log ec2 stays aws",
			usage:           "689860182975",
			serviceType:     "Amazon Elastic Compute Cloud",
			billingEntity:   "AWS",
			wantMarketplace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isYCloudMarketplaceCost(tt.usage, tt.serviceType, tt.billingEntity)
			if got != tt.wantMarketplace {
				t.Fatalf("isYCloudMarketplaceCost() = %v, want %v", got, tt.wantMarketplace)
			}
		})
	}
}
