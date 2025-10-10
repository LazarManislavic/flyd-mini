<%@ Page Title="" Language="C#" MasterPageFile="~/Site.Master" AutoEventWireup="true" CodeBehind="Akcije_mng.aspx.cs" Inherits="WebApplication6.Account.Akcije_mng" %>
<asp:Content ID="Content1" ContentPlaceHolderID="MainContent" runat="server">

    <asp:GridView ID="GridView2" runat="server" AutoGenerateColumns="False" DataSourceID="SqlDataSource1">
        <Columns>
            <asp:BoundField DataField="Sifra_proizvoda" HeaderText="Sifra_proizvoda" SortExpression="Sifra_proizvoda"></asp:BoundField>
            <asp:BoundField DataField="Akciska_cena" HeaderText="Akciska_cena" SortExpression="Akciska_cena"></asp:BoundField>
        </Columns>
    </asp:GridView>
    <asp:SqlDataSource runat="server" ID="SqlDataSource1" ConnectionString="<%$ ConnectionStrings:lazarConnectionString %>" SelectCommand="SELECT [Sifra_proizvoda], [Akciska_cena] FROM [akcije]"></asp:SqlDataSource><br />
    <asp:Label ID="Label1" runat="server" Text="Izaberi sifru akcije
        "></asp:Label>
    <asp:DropDownList ID="DropDownList1" runat="server" DataTextField="Sifra" DataValueField="Sifra" DataSourceID="SqlDataSource2"></asp:DropDownList><asp:SqlDataSource runat="server" ID="SqlDataSource2" ConnectionString="<%$ ConnectionStrings:lazarConnectionString %>" SelectCommand="SELECT [Sifra] FROM [akcije]"></asp:SqlDataSource><br />
    <asp:Label ID="Label2" runat="server" Text="Akciska cena NOVA:"></asp:Label><asp:TextBox ID="TextBox1" runat="server"></asp:TextBox><br />
    <asp:Button ID="Button1" runat="server" Text="Promeni cenu" OnClick="Button1_Click" /><asp:Label ID="ErrorLabel1" runat="server" Text="Label"></asp:Label>


</asp:Content>
