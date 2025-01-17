package msgpipeline

import (
	"testing"

	"github.com/foxcpp/maddy/framework/module"
	"github.com/foxcpp/maddy/internal/testutils"
)

func TestMsgPipeline_Issue161(t *testing.T) {
	target := testutils.Target{}
	check1, check2 := testutils.Check{}, testutils.Check{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&check1},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				checks:  []module.Check{&check2},
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if check2.ConnCalls != 1 {
		t.Errorf("CheckConnection called %d times", check2.ConnCalls)
	}
	if check2.SenderCalls != 1 {
		t.Errorf("CheckSender called %d times", check2.SenderCalls)
	}
	if check2.RcptCalls != 1 {
		t.Errorf("CheckRcpt called %d times", check2.RcptCalls)
	}
	if check2.BodyCalls != 1 {
		t.Errorf("CheckBody called %d times", check2.BodyCalls)
	}

	if check1.UnclosedStates != 0 || check2.UnclosedStates != 0 {
		t.Fatalf("checks state objects leak or double-closed, alive counters: %v, %v", check1.UnclosedStates, check2.UnclosedStates)
	}
}

func TestMsgPipeline_Issue161_2(t *testing.T) {
	target := testutils.Target{}
	check1, check2 := testutils.Check{}, testutils.Check{InstName: "check2"}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&check1},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				checks:  []module.Check{&check1},
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					checks:  []module.Check{&check2},
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if check2.ConnCalls != 1 {
		t.Errorf("CheckConnection called %d times", check2.ConnCalls)
	}
	if check2.SenderCalls != 1 {
		t.Errorf("CheckSender called %d times", check2.SenderCalls)
	}
	if check2.RcptCalls != 1 {
		t.Errorf("CheckRcpt called %d times", check2.RcptCalls)
	}

	if check1.UnclosedStates != 0 || check2.UnclosedStates != 0 {
		t.Fatalf("checks state objects leak or double-closed, alive counters: %v, %v", check1.UnclosedStates, check2.UnclosedStates)
	}
}

func TestMsgPipeline_Issue161_3(t *testing.T) {
	target := testutils.Target{}
	check1, check2 := testutils.Check{}, testutils.Check{}
	d := MsgPipeline{
		msgpipelineCfg: msgpipelineCfg{
			globalChecks: []module.Check{&check1, &check2},
			perSource:    map[string]sourceBlock{},
			defaultSource: sourceBlock{
				perRcpt: map[string]*rcptBlock{},
				defaultRcpt: &rcptBlock{
					targets: []module.DeliveryTarget{&target},
				},
			},
		},
		Log: testutils.Logger(t, "msgpipeline"),
	}

	testutils.DoTestDelivery(t, &d, "whatever@whatever", []string{"whatever@whatever"})

	if check2.ConnCalls != 1 {
		t.Errorf("CheckConnection called %d times", check2.ConnCalls)
	}
	if check2.SenderCalls != 1 {
		t.Errorf("CheckSender called %d times", check2.SenderCalls)
	}
	if check2.RcptCalls != 1 {
		t.Errorf("CheckRcpt called %d times", check2.RcptCalls)
	}
	if check2.BodyCalls != 1 {
		t.Errorf("CheckBody called %d times", check2.BodyCalls)
	}

	if check1.UnclosedStates != 0 || check2.UnclosedStates != 0 {
		t.Fatalf("checks state objects leak or double-closed, alive counters: %v, %v", check1.UnclosedStates, check2.UnclosedStates)
	}
}
