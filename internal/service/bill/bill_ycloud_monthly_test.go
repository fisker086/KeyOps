package bill

import "testing"

func TestBuildYCloudMonthlyBill_May2026(t *testing.T) {
	gross := 527155.07
	excluded := 30489.51
	bill := buildYCloudMonthlyBill(gross, excluded, defaultEDPDiscountRate)

	if bill.GrossTotal != gross {
		t.Fatalf("gross = %v", bill.GrossTotal)
	}
	wantDiscount := gross * defaultEDPDiscountRate
	if bill.ServiceDiscount < wantDiscount-1 || bill.ServiceDiscount > wantDiscount+1 {
		t.Fatalf("discount = %v, want ~%v", bill.ServiceDiscount, wantDiscount)
	}
	wantNet := gross - bill.ServiceDiscount
	if bill.NetServiceCost < wantNet-1 || bill.NetServiceCost > wantNet+1 {
		t.Fatalf("net = %v, want ~%v", bill.NetServiceCost, wantNet)
	}
	wantSupport := excluded * ycloudSupportBillableFactor
	if bill.SupportBillable < wantSupport-1 || bill.SupportBillable > wantSupport+1 {
		t.Fatalf("support = %v, want ~%v", bill.SupportBillable, wantSupport)
	}
	wantTotal := bill.NetServiceCost + bill.SupportBillable
	if bill.MonthlyBillTotal < wantTotal-1 || bill.MonthlyBillTotal > wantTotal+1 {
		t.Fatalf("total = %v, want ~%v", bill.MonthlyBillTotal, wantTotal)
	}
	if bill.MonthlyBillTotal < 480000 {
		t.Fatalf("monthly total too low: %v", bill.MonthlyBillTotal)
	}
}
